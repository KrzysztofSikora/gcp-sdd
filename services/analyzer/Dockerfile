FROM python:3.13-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /srv
COPY pyproject.toml ./
COPY app ./app
RUN pip install . && useradd --system --uid 10001 --no-create-home analyzer

USER 10001
ENV PORT=8080
EXPOSE 8080
# Run through sh so that Cloud Run's $PORT is expanded; exec keeps uvicorn as PID 1 for SIGTERM.
CMD ["sh", "-c", "exec uvicorn app.main:app --host 0.0.0.0 --port \"$PORT\""]
