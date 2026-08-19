FROM ghcr.io/astral-sh/uv:0.10.0 AS uv

FROM python:3.14.3-slim AS runtime
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV UV_COMPILE_BYTECODE=1
ENV UV_LINK_MODE=copy
WORKDIR /app
COPY --from=uv /uv /uvx /bin/
COPY workers/evaluator/pyproject.toml workers/evaluator/uv.lock ./workers/evaluator/
COPY workers/evaluator/src ./workers/evaluator/src
COPY packages/rule-packs ./packages/rule-packs
RUN uv sync --project workers/evaluator --locked --no-dev \
    && addgroup --system --gid 10001 flightcheck \
    && adduser --system --uid 10001 --ingroup flightcheck flightcheck \
    && chown -R flightcheck:flightcheck /app
USER flightcheck
ENTRYPOINT ["uv", "run", "--project", "workers/evaluator", "--no-sync", "flightcheck-evaluator"]
CMD ["worker"]
