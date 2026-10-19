import os
import sys
from pathlib import Path
from urllib.parse import urlparse, urlunparse

import psycopg2
import pytest
from dotenv import load_dotenv

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "worker"))   # the worker uses flat imports (import db, executor)
load_dotenv(ROOT.parent / ".env")


def _test_db_url() -> str:
    url = os.environ.get("TEST_DATABASE_URL")
    if not url:
        u = urlparse(os.environ["DATABASE_URL"])
        url = urlunparse(u._replace(path=u.path.rstrip("/") + "_test"))
    # these tests TRUNCATE everything, same guard as the Go tests
    assert urlparse(url).path.endswith("_test"), f"refusing to run against {url}: name must end in _test"
    return url


@pytest.fixture
def conn():
    try:
        c = psycopg2.connect(_test_db_url())
    except psycopg2.OperationalError as e:
        pytest.skip(f"postgres test database unavailable (run the Go tests once to create it): {e}")
    with c.cursor() as cur:
        cur.execute("TRUNCATE dags CASCADE")
    c.commit()
    yield c
    c.close()


@pytest.fixture
def cur(conn):
    with conn.cursor() as cursor:
        yield cursor


@pytest.fixture
def pool(conn):
    from psycopg2 import pool as pg_pool
    p = pg_pool.SimpleConnectionPool(1, 3, _test_db_url())
    yield p
    p.closeall()
