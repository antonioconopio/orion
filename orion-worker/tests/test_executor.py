import json
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


# ---------- task outputs and inputs ----------

WRITE = "import json, os; json.dump({}, open(os.environ['ORION_OUTPUT'], 'w'))"


@pytest.mark.parametrize("value", [{"rows": 3, "tags": ["a", "b"]}, [1, 2, 3], 42, "text", True, {"nested": {"x": [1, {"y": None}]}}, {"é": "ünïcode ✓"}])
def test_a_tasks_json_output_is_returned(value):
    r = run(WRITE.format(repr(value)))
    assert r.success and r.output == value


def test_no_output_file_means_no_output():
    r = run("print('hi')")
    assert r.success and r.output is None


@pytest.mark.parametrize("content", ["", "   \n", "null"])
def test_an_empty_or_null_output_means_no_output(content):
    r = run(f"open(__import__('os').environ['ORION_OUTPUT'], 'w').write({content!r})")
    assert r.success and r.output is None


@pytest.mark.parametrize("content,reason", [
    ("{not json", "not valid JSON"),
    ('{"n": NaN}', "NaN is not valid JSON"),
    ('[Infinity]', "Infinity is not valid JSON"),
    ('"\\u0000"', "null character"),
    ("[" * 30000, "not valid JSON"),
])
def test_output_that_breaks_the_contract_fails_the_task(content, reason):
    r = run(f"open(__import__('os').environ['ORION_OUTPUT'], 'w').write({content!r})")
    assert not r.success and r.output is None and reason in r.stderr
    assert r.returncode == 0   # the script itself exited cleanly; the output is what was wrong


def test_invalid_utf8_output_fails_the_task():
    r = run("open(__import__('os').environ['ORION_OUTPUT'], 'wb').write(b'\\xff\\xfe')")
    assert not r.success and "not valid JSON" in r.stderr


def test_an_oversized_output_fails_the_task():
    r = run(f"open(__import__('os').environ['ORION_OUTPUT'], 'w').write('\"' + 'x' * {executor.MAX_RESULT_BYTES} + '\"')")
    assert not r.success and "byte limit" in r.stderr


def test_output_just_under_the_limit_is_accepted():
    n = executor.MAX_RESULT_BYTES - 2
    r = run(f"open(__import__('os').environ['ORION_OUTPUT'], 'w').write('\"' + 'x' * {n} + '\"')")
    assert r.success and len(r.output) == n


def test_a_failed_task_publishes_nothing():
    r = run(WRITE.format({"half": "done"}) + "; raise SystemExit(1)")
    assert not r.success and r.output is None


def test_a_timed_out_task_publishes_nothing():
    r = run(WRITE.format({"a": 1}) + "; import time; time.sleep(30)", timeout=1)
    assert r.timed_out and r.output is None


def test_a_task_reads_its_parents_outputs_from_orion_inputs():
    inputs = {"extract": {"n": 3}, "clean": None}
    r = executor.execute("python", {"script": "import json, os; print(json.dumps(json.load(open(os.environ['ORION_INPUTS'])), sort_keys=True))"}, None, inputs)
    assert json.loads(r.stdout) == inputs


def test_a_task_without_parents_gets_an_empty_object():
    r = run("import json, os; print(json.load(open(os.environ['ORION_INPUTS'])))")
    assert r.stdout.strip() == "{}"


def test_inputs_larger_than_an_environment_variable_still_work():
    big = {"a": "x" * 60_000, "b": "y" * 60_000, "c": "z" * 60_000}   # 180 KB in total, over Linux's 128 KB per-variable limit
    r = executor.execute("python", {"script": "import json, os; print(len(json.load(open(os.environ['ORION_INPUTS']))))"}, None, big)
    assert r.success and r.stdout.strip() == "3"


def test_a_tasks_own_env_cannot_override_the_orion_paths():
    r = run("import os; print(os.environ['ORION_OUTPUT'])", env={"ORION_OUTPUT": "/tmp/evil", "ORION_INPUTS": "/tmp/evil"})
    assert r.stdout.strip() != "/tmp/evil" and "orion-" in r.stdout


def test_the_temporary_files_are_removed_afterwards():
    r = run("import os; print(os.environ['ORION_OUTPUT'])")
    assert not os.path.exists(os.path.dirname(r.stdout.strip()))


def test_the_temporary_files_are_removed_even_when_the_task_fails():
    r = run("import os; print(os.environ['ORION_OUTPUT']); raise SystemExit(2)")
    assert not r.success and not os.path.exists(os.path.dirname(r.stdout.strip()))
