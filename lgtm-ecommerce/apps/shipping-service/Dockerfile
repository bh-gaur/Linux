FROM python:3.11-slim

WORKDIR /app

COPY shipping-service/requirements.txt ./shipping-service/

RUN pip install --no-cache-dir -r ./shipping-service/requirements.txt && \
    opentelemetry-bootstrap --action=install

COPY common ./common
COPY shipping-service/ ./shipping-service/

WORKDIR /app/shipping-service

EXPOSE 8097

CMD ["gunicorn", "--workers=2", "--threads=2", "--worker-class=gthread", "--bind=0.0.0.0:8097", "--timeout=60", "app:app"]
