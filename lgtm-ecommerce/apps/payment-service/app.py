import sys
import os
import uuid
import time
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from common.traces import setup_observability, get_baggage_value, attach_baggage_to_span, trace
from common.logger import logger
from common.metrics import analysis_counter, task_duration_histogram, payment_transactions_total



from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="Payment Service")
setup_observability(app)

class ChargeRequest(BaseModel):
    user_id: str
    amount: float
    order_id: str

@app.post("/charge")
def charge_payment(payload: ChargeRequest):
    # Retrieve active span
    active_span = trace.get_current_span()
    attach_baggage_to_span(active_span)

    # Get baggage value
    tier = get_baggage_value("customer.tier") or "regular"
    amount = payload.amount
    discount_applied = False

    if tier == "gold":
        amount = amount * 0.9  # 10% Gold Discount
        discount_applied = True
        active_span.set_attribute("payment.discount_applied", True)
        logger.info(f"Gold Tier customer detected! Applying 10% discount: ${payload.amount:.2f} -> ${amount:.2f}")

    logger.info(f"Processing charging transaction for user '{payload.user_id}' with amount ${amount:.2f}")

    start_time = time.time()
    time.sleep(0.05)

    if amount <= 0:
        logger.error(f"Charging transaction rejected: negative amount {amount}")
        raise HTTPException(status_code=400, detail="Invalid charging amount")

    txn_id = f"txn_{uuid.uuid4().hex[:12]}"
    duration = time.time() - start_time

    analysis_counter.add(1, {"operation": "payment_charge", "status": "success"})
    task_duration_histogram.record(duration, {"task_name": "payment_charge"})
    payment_transactions_total.add(1, {"tier": tier, "status": "success"})

    logger.info(f"Charged ${amount:.2f} successfully. Transaction ID: {txn_id}")
    return {"status": "charged", "transaction_id": txn_id, "amount": amount}

@app.get("/health")
def health():
    return {"status": "healthy", "service": "payment-service"}
