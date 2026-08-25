import os
import sys
import json
import threading
import time
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from common.traces import setup_observability, trace, attach_baggage_to_span
from common.logger import logger



from fastapi import FastAPI
from kafka import KafkaConsumer

app = FastAPI(title="Notification Service")
setup_observability(app)

tracer = trace.get_tracer("notification-service-tracer")

@app.get("/health")
def health():
    return {"status": "healthy", "service": "notification-service"}

def start_kafka_consumer():
    kafka_brokers = os.getenv("KAFKA_BROKERS", "kafka:9092")

    def consume_loop():
        time.sleep(10) # wait for kafka to boot
        logger.info(f"Connecting notification-service consumer to brokers: {kafka_brokers}")

        while True:
            try:
                service_name_val = os.getenv("OTEL_SERVICE_NAME", "notification-service")
                consumer = KafkaConsumer(
                    "order-events",
                    bootstrap_servers=kafka_brokers.split(","),
                    group_id=f"{service_name_val}-group",
                    auto_offset_reset="latest",
                    value_deserializer=lambda x: json.loads(x.decode("utf-8"))
                )
                logger.info("Notification Kafka Consumer connected successfully.")

                from opentelemetry import propagate

                for msg in consumer:
                    event = msg.value
                    logger.info(f"Notification service consumed order completion event: {event}")

                    # Extract trace context
                    headers_dict = {}
                    if msg.headers:
                        for k, v in msg.headers:
                            try:
                                headers_dict[k] = v.decode('utf-8') if isinstance(v, bytes) else str(v)
                            except Exception:
                                pass

                    parent_context = propagate.extract(headers_dict)

                    with tracer.start_as_current_span("ProcessNotificationEvent", context=parent_context) as span:
                        attach_baggage_to_span(span)
                        span.set_attribute("order.id", event.get("order_id", "unknown"))
                        logger.info(f"[EMAIL-DISPATCH] Dispatched order confirmation email to customer for Order: {event.get('order_id')}")

            except Exception as e:
                logger.warning(f"Kafka consumer connection retry in 5s: {e}")
                time.sleep(5)

    t = threading.Thread(target=consume_loop, daemon=True)
    t.start()

start_kafka_consumer()

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8098)
