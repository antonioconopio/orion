import os
import time

import pytest

import executor


def run(script, **config):
    return executor.execute("python", {"script": script, **config})


def test_success_captures_stdout():
    r = run("print('hi')")
    assert r.success and r.returncode == 0 and r.stdout == "hi\n" and not r.timed_out


def test_exception_fails_with_traceback():
    r = run("raise ValueError('boom')")
    assert not r.success and r.returncode == 1 and "ValueError: boom" in r.stderr


def test_timeout_is_reported():
    r = run("import time; time.sleep(30)", timeout=1)
    assert not r.success and r.timed_out and r.returncode is None


def test_timeout_keeps_output_printed_before_the_kill():
    r = run("print('before', flush=True); import time; time.sleep(30)", timeout=1)
    assert r.timed_out and "before" in r.stdout


def test_timeout_kills_processes_the_script_spawned():
    r = run("import subprocess, time\n"
            "p = subprocess.Popen(['sleep', '30'])\n"
            "print(p.pid, flush=True)\n"
            "time.sleep(30)", timeout=1)
    pid = int(r.stdout.split()[0])
    for _ in range(20):
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            return
        time.sleep(0.1)
    pytest.fail(f"grandchild {pid} survived the timeout")


def test_timeout_is_capped(monkeypatch):
    monkeypatch.setattr(executor, "MAX_TIMEOUT", 1)
    start = time.monotonic()
    r = run("import time; time.sleep(30)", timeout=1000)
    assert r.timed_out and time.monotonic() - start < 10


@pytest.mark.parametrize("bad", ["x", True, 0, -5, None, [1]])
def test_bad_timeout_fails_cleanly(bad):
    r = run("print(1)", timeout=bad)
    assert not r.success and "timeout" in r.stderr


@pytest.mark.parametrize("bad", [[], "a=b", 3])
def test_bad_env_fails_cleanly(bad):
    r = run("print(1)", env=bad)
    assert not r.success and "env" in r.stderr


def test_missing_script_fails_cleanly():
    r = executor.execute("python", {})
    assert not r.success and "script" in r.stderr


def test_env_is_passed_and_the_workers_own_env_is_not(monkeypatch):
    monkeypatch.setenv("DATABASE_URL", "postgres://secret")
    r = run("import os; print(os.environ.get('MY_VAR'), os.environ.get('DATABASE_URL'))", env={"MY_VAR": 7})
    assert r.stdout.strip() == "7 None"


def test_output_is_truncated():
    r = run(f"print('x' * {executor.MAX_OUTPUT * 3})")
    assert len(r.stdout) == executor.MAX_OUTPUT


@pytest.mark.parametrize("version", ["3", "3.", "2.7", "3.11.2", "python3.11", "3.x", "../3.11"])
def test_invalid_python_version_is_rejected(version):
    r = executor.execute("python", {"script": "print(1)"}, version)
    assert not r.success and "invalid python_version" in r.stderr


def test_missing_interpreter_fails_cleanly():
    r = executor.execute("python", {"script": "print(1)"}, "3.99")
    assert not r.success and "not installed" in r.stderr


def test_unknown_task_type_fails_cleanly():
    r = executor.execute("shell", {"script": "ls"})
    assert not r.success and "unknown task_type" in r.stderr
