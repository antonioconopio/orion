import json, os, signal, subprocess, sys, re, shutil, tempfile
from dataclasses import dataclass
from typing import Any

DEFAULT_TIMEOUT = 60      # seconds
MAX_TIMEOUT = 100         # main.py derives MIN_IDLE_MS from this
MAX_OUTPUT = 10_000       # characters kept from stdout/stderr
MAX_RESULT_BYTES = 64 * 1024   # a task's JSON result: it is stored in Postgres and re-read by every child


@dataclass
class ExecResult:
    success: bool
    returncode: int | None
    stdout: str
    stderr: str
    timed_out: bool = False
    output: Any = None   # the JSON the task wrote to ORION_OUTPUT (only set when it succeeded)


def _reject_constant(name: str):
    raise ValueError(f"{name} is not valid JSON")


def read_result(path: str) -> tuple[Any, str | None]:
    """What a task wrote to ORION_OUTPUT: (value, None), or (None, reason) if it broke the contract.
    A missing or empty file is normal: the task simply has no output."""
    try:
        size = os.path.getsize(path)
    except FileNotFoundError:
        return None, None
    if size > MAX_RESULT_BYTES:
        return None, f"task output is {size} bytes, over the {MAX_RESULT_BYTES} byte limit"
    with open(path, "rb") as f:
        raw = f.read()
    if not raw.strip():
        return None, None
    try:
        value = json.loads(raw, parse_constant=_reject_constant)   # rejects NaN and Infinity, which jsonb cannot store
    except Exception as e:   # bad JSON, bad UTF-8, or nesting too deep
        return None, f"task output is not valid JSON: {e}"
    # Postgres jsonb refuses \u0000. Catching it here beats a failed write after the task ran, which
    # would leave the instance 'running' and have crash recovery re-run it forever.
    if "\\u0000" in json.dumps(value):
        return None, "task output contains a null character (\\u0000), which Postgres cannot store"
    return value, None


def run_python(config: dict, version: str | None, inputs: dict | None = None) -> ExecResult:
    script = config.get("script")

    if not script:
        return ExecResult(False, None, "", "task config has no 'script'")

    raw_env = config.get("env", {})
    if not isinstance(raw_env, dict):
        return ExecResult(False, None, "", "task config 'env' must be an object")

    raw_timeout = config.get("timeout", DEFAULT_TIMEOUT)
    if isinstance(raw_timeout, bool) or not isinstance(raw_timeout, (int, float)) or raw_timeout <= 0:
        return ExecResult(False, None, "", "task config 'timeout' must be a positive number")

    base_env = {"PATH": os.environ.get("PATH", ""), "PYTHONUNBUFFERED": "1"}
    user_env = {str(k): str(v) for k, v in raw_env.items()}

    if version is None:
        interpreter = sys.executable
    else:
        if not re.fullmatch(r"3\.\d{1,2}", str(version)):
            return ExecResult(False, None, "", f"invalid python_version {version!r}")
        interpreter = shutil.which(f"python{version}")
        if interpreter is None:
            return ExecResult(False, None, "", f"python{version} is not installed on this worker")

    timeout = min(raw_timeout, MAX_TIMEOUT)

    # The task reads its parents' outputs from ORION_INPUTS and publishes its own to ORION_OUTPUT.
    # Both are files (an environment variable is capped at 128 KB on Linux, which a few parent
    # outputs would exceed), in a private directory that is always removed afterwards.
    with tempfile.TemporaryDirectory(prefix="orion-") as workdir:
        inputs_path = os.path.join(workdir, "inputs.json")
        output_path = os.path.join(workdir, "output.json")
        with open(inputs_path, "w") as f:
            json.dump(inputs or {}, f)
        # ORION_* go last so a task's own env config cannot override them
        env = {**base_env, **user_env, "ORION_INPUTS": inputs_path, "ORION_OUTPUT": output_path}

        try:
            proc = subprocess.Popen(
                [interpreter, "-c", script],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                env=env,
                start_new_session=True,       # own process group, so a timeout can kill the script's children too
            )
        except OSError as e:
            return ExecResult(False, None, "", f"failed to start: {e}")

        try:
            stdout, stderr = proc.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid, signal.SIGKILL)
            try:
                stdout, stderr = proc.communicate(timeout=5)
            except subprocess.TimeoutExpired:   # shortcut: a child that left the group can hold the pipes open, output is lost then
                stdout, stderr = "", ""
            return ExecResult(False, None, stdout[:MAX_OUTPUT], stderr[:MAX_OUTPUT], timed_out=True)

        if proc.returncode != 0:   # a failed task publishes nothing
            return ExecResult(False, proc.returncode, stdout[:MAX_OUTPUT], stderr[:MAX_OUTPUT])

        value, problem = read_result(output_path)
        if problem:
            stderr = f"{stderr}\n{problem}" if stderr else problem
            return ExecResult(False, proc.returncode, stdout[:MAX_OUTPUT], stderr[:MAX_OUTPUT])
        return ExecResult(True, proc.returncode, stdout[:MAX_OUTPUT], stderr[:MAX_OUTPUT], output=value)


EXECUTORS = {"python": run_python}


def execute(task_type: str, config: dict, python_version: str | None = None, inputs: dict | None = None) -> ExecResult:
    fn = EXECUTORS.get(task_type)
    if fn is None:
        return ExecResult(False, None, "", f"unknown task_type {task_type!r}")
    return fn(config, python_version, inputs)
