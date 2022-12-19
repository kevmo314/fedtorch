FROM nvidia/cuda:12.0.0-base-ubuntu22.04 AS base

ENV PYTHONFAULTHANDLER=1 \
    PYTHONHASHSEED=random \
    PYTHONUNBUFFERED=1
ENV POETRY_VENV=/opt/poetry-venv

RUN apt update && apt install -y python3 python3-venv

WORKDIR /app

FROM base as builder

ENV POETRY_VERSION=1.2.0
ENV POETRY_HOME=/opt/poetry
ENV POETRY_CACHE_DIR=/opt/.cache

RUN python3 -m venv $POETRY_VENV \
    && $POETRY_VENV/bin/pip install -U pip setuptools \
    && $POETRY_VENV/bin/pip install poetry==${POETRY_VERSION}

ENV PATH="${PATH}:${POETRY_VENV}/bin"

COPY poetry.lock pyproject.toml ./
RUN poetry install

COPY . .
RUN poetry build && $POETRY_VENV/bin/pip install dist/*.whl

FROM base as final

COPY --from=builder $POETRY_VENV $POETRY_VENV
COPY docker-entrypoint.sh wsgi.py ./

EXPOSE 5000

CMD ["./docker-entrypoint.sh"]