import sys
import os
import psycopg2
import time
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from common.traces import setup_observability
from common.logger import logger
from common.metrics import analysis_counter, task_duration_histogram


from flask import Flask, jsonify

app = Flask(__name__)
setup_observability(app)

db_connected = False
db_conn = None

def get_db_conn():
    global db_connected, db_conn
    if db_connected and db_conn is not None:
        try:
            # Check connection status
            with db_conn.cursor() as cur:
                cur.execute("SELECT 1")
            return db_conn
        except Exception:
            db_connected = False
            db_conn = None

    db_host = os.getenv("DB_HOST", "postgres")
    try:
        db_conn = psycopg2.connect(
            host=db_host,
            database="lgtmdb",
            user="lgtm_user",
            password="lgtm_password",
            port="5432"
        )
        db_connected = True
        logger.info("Reporting service connected successfully to database")
        return db_conn
    except Exception as e:
        db_connected = False
        db_conn = None
        logger.warning(f"Database connection offline in reporting-service: {e}")
        return None

@app.route("/report/summary", methods=["GET"])
def report_summary():
    logger.info("Received request for database summary statistics")

    start_time = time.time()
    conn = get_db_conn()
    duration = time.time() - start_time

    analysis_counter.add(1, {"operation": "db_summary", "status": "success" if conn else "failed"})
    task_duration_histogram.record(duration, {"task_name": "db_summary"})

    if not conn:
        return jsonify({
            "status": "warning",
            "message": "Database offline, summary unavailable",
            "metrics": {"total_orders": 0, "active_tables": ["inventory"]}
        })

    try:
        with conn.cursor() as cur:
            # Count orders
            cur.execute("SELECT COUNT(*) FROM orders")
            total_orders = cur.fetchone()[0]

            # Count total sales revenue
            cur.execute("SELECT COALESCE(SUM(total_price), 0.0) FROM orders")
            total_revenue = float(cur.fetchone()[0])

            logger.info(f"Summary computed: {total_orders} orders, ${total_revenue:.2f} total revenue")
            return jsonify({
                "status": "success",
                "metrics": {
                    "total_orders": total_orders,
                    "total_revenue": total_revenue
                }
            })
    except Exception as e:
        logger.error(f"Error executing reporting query: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500

@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "service": "reporting-service"})

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8099)
