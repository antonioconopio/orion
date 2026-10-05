import os, subprocess, sys, re, shutil
from dataclasses import dataclass

DEFAULT_TIMEOUT = 60      # seconds
MAX_TIMEOUT = 100         # must stay under MIN_IDLE_MS / 1000
MAX_OUTPUT = 10_000       # characters kept from stdout/stderr


@dataclass
class ExecResult:
    success: bool
    returncode: int | None
    stdout: str
    stderr: str
    timed_out: bool = False

def _text(value) -> str:
    if value is None:
        return ""
    if isinstance(value, bytes):
        return value.decode(errors="replace")
    return value


def run_python(config: dict, version: str) -> ExecResult:
    script = config.get("script")
    
    if not script:
        return ExecResult(False, None, "", "task config has no 'script'")

    base_env = {"PATH": os.environ.get("PATH", ""), "PYTHONUNBUFFERED": "1"}
    user_env = {str(k): str(v) for k, v in config.get("env", {}).items()}
    env = {**base_env, **user_env}

    if version is None:
        interpreter = sys.executable
    else:
        if not re.fullmatch(r"3\.\d{1,2}", str(version)):
            return ExecResult(False, None, "", f"invalid python_version {version!r}")
        interpreter = shutil.which(f"python{version}")
        if interpreter is None:
            return ExecResult(False, None, "", f"python{version} is not installed on this worker")

    timeout = min(config.get("timeout", DEFAULT_TIMEOUT), MAX_TIMEOUT)

    try:
        proc = subprocess.run(
            [interpreter, "-c", script],           
            capture_output=True,
            text=True,
            timeout=timeout,
            env=env,                      
        )
    except subprocess.TimeoutExpired as e:
        return ExecResult(False, None, _text(e.stdout)[:MAX_OUTPUT],_text(e.stderr)[:MAX_OUTPUT], timed_out=True)
    except OSError as e:
        return ExecResult(False, None, "", f"failed to start: {e}")

    return ExecResult(
        success=proc.returncode == 0,
        returncode=proc.returncode,
        stdout=proc.stdout[:MAX_OUTPUT],
        stderr=proc.stderr[:MAX_OUTPUT],
    )


EXECUTORS = {"python": run_python}


def execute(task_type: str, config: dict, python_version: str | None = None) -> ExecResult:
    fn = EXECUTORS.get(task_type)
    if fn is None:
        return ExecResult(False, None, "", f"unknown task_type {task_type!r}")
    return fn(config, python_version)