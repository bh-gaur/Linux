FROM python:3.11-slim

WORKDIR /app

COPY payment-service/requirements.txt ./payment-service/

RUN pip install --no-cache-dir -r ./payment-service/requirements.txt && \
    opentelemetry-bootstrap --action=install

COPY common ./common
COPY payment-service/ ./payment-service/

WORKDIR /app/payment-service

EXPOSE 8096

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8096"]
