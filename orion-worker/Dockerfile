FROM python:3.12-slim

# DAGs can pin python_version (e.g. "3.11"); the worker runs `python3.11` from PATH.
# 3.12 comes from the base image, uv adds the rest.
COPY --from=ghcr.io/astral-sh/uv:0.5.11 /uv /usr/local/bin/uv
ENV UV_PYTHON_INSTALL_DIR=/opt/uv-python
RUN uv python install 3.10 3.11 3.13 \
 && for v in 3.10 3.11 3.13; do ln -s "$(uv python find $v)" /usr/local/bin/python$v; done

RUN useradd --system --create-home worker

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY worker/ worker/
USER worker
WORKDIR /app/worker

# main.py uses flat imports, so it must run from this directory
CMD ["python", "main.py"]
