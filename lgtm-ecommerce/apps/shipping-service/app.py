import sys
import os
import random
import time
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from common.traces import setup_observability, attach_baggage_to_span, trace
from common.logger import logger
from common.metrics import analysis_counter, task_duration_histogram


from flask import Flask, request, jsonify

app = Flask(__name__)
setup_observability(app)

@app.route("/ship", methods=["POST"])
def ship_order():
    # Retrieve active span and attach baggage attributes
    active_span = trace.get_current_span()
    attach_baggage_to_span(active_span)

    payload = request.get_json() or {}
    order_id = payload.get("order_id", "unknown_order")
    user_id = payload.get("user_id", "unknown_user")

    logger.info(f"Generating shipping label for order {order_id} (user: {user_id})")

    start_time = time.time()
    time.sleep(0.04) # Simulate label generator delay

    # Generate mock label
    label_num = random.randint(100000, 999999)
    label = f"lbl_{label_num}US"
    duration = time.time() - start_time

    analysis_counter.add(1, {"operation": "generate_shipping_label", "status": "success"})
    task_duration_histogram.record(duration, {"task_name": "generate_shipping_label"})

    logger.info(f"Shipping label {label} generated successfully for order {order_id}")
    return jsonify({"status": "shipped", "tracking_label": label, "order_id": order_id})

@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "service": "shipping-service"})

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8097)
