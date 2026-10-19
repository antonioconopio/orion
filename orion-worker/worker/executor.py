import os, signal, subprocess, sys, re, shutil
from dataclasses import dataclass

DEFAULT_TIMEOUT = 60      # seconds
MAX_TIMEOUT = 100         # main.py derives MIN_IDLE_MS from this
MAX_OUTPUT = 10_000       # characters kept from stdout/stderr


@dataclass
class ExecResult:
    success: bool
    returncode: int | None
    stdout: str
    stderr: str
    timed_out: bool = False

def run_python(config: dict, version: str | None) -> ExecResult:
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
    env = {**base_env, **user_env}

    if version is None:
        interpreter = sys.executable
    else:
        if not re.fullmatch(r"3\.\d{1,2}", str(version)):
            return ExecResult(False, None, "", f"invalid python_version {version!r}")
        interpreter = shutil.which(f"python{version}")
        if interpreter is None:
            return ExecResult(False, None, "", f"python{version} is not installed on this worker")

    timeout = min(raw_timeout, MAX_TIMEOUT)

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

    return ExecResult(
        success=proc.returncode == 0,
        returncode=proc.returncode,
        stdout=stdout[:MAX_OUTPUT],
        stderr=stderr[:MAX_OUTPUT],
    )


EXECUTORS = {"python": run_python}


def execute(task_type: str, config: dict, python_version: str | None = None) -> ExecResult:
    fn = EXECUTORS.get(task_type)
    if fn is None:
        return ExecResult(False, None, "", f"unknown task_type {task_type!r}")
    return fn(config, python_version)