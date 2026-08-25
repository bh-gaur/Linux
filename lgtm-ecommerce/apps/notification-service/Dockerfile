FROM python:3.11-slim

WORKDIR /app

COPY notification-service/requirements.txt ./notification-service/

RUN pip install --no-cache-dir -r ./notification-service/requirements.txt && \
    opentelemetry-bootstrap --action=install

COPY common ./common
COPY notification-service/ ./notification-service/

WORKDIR /app/notification-service

EXPOSE 8098

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8098"]
