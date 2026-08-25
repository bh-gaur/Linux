FROM python:3.11-slim

WORKDIR /app

COPY reporting-service/requirements.txt ./reporting-service/

RUN pip install --no-cache-dir -r ./reporting-service/requirements.txt && \
    opentelemetry-bootstrap --action=install

COPY common ./common
COPY reporting-service/ ./reporting-service/

WORKDIR /app/reporting-service

EXPOSE 8099

CMD ["gunicorn", "--workers=2", "--threads=2", "--worker-class=gthread", "--bind=0.0.0.0:8099", "--timeout=60", "app:app"]
