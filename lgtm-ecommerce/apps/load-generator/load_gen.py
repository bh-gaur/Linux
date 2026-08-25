import time
import random
import requests

GATEWAY_URL = "http://gateway-service:8090"
API_KEY = "lgtm-secret-key"

def run_simulator():
    print("[*] E-Commerce Load Generator started. Waiting 15 seconds for stack startup...", flush=True)
    time.sleep(15)
    
    users = ["Alice", "Bob", "Charlie", "Dave", "Eve"]
    
    while True:
        try:
            # 1. Randomly select a user
            user = random.choice(users)
            user_id = f"user_{user.lower()}"
            print(f"\n[*] Simulating traffic for user: {user}", flush=True)

            headers = {
                "x-api-key": API_KEY,
                "Content-Type": "application/json"
            }

            # 2. Login to retrieve bearer token
            print(f"-> Logging in user '{user}'...", flush=True)
            login_resp = requests.post(
                f"{GATEWAY_URL}/login",
                headers=headers,
                json={"username": user, "password": "secure_password_123"},
                timeout=5
            )
            if login_resp.status_code != 200:
                print(f"[!] Login failed: {login_resp.status_code}", flush=True)
                time.sleep(5)
                continue
                
            token = login_resp.json().get("token")
            headers["authorization"] = token

            # 3. Retrieve Product Catalog
            print("-> Retrieving products catalog...", flush=True)
            catalog_resp = requests.get(f"{GATEWAY_URL}/catalog", headers=headers, timeout=5)
            if catalog_resp.status_code != 200:
                print(f"[!] Catalog fetch failed: {catalog_resp.status_code}", flush=True)
                time.sleep(5)
                continue
                
            catalog = catalog_resp.json()
            if not catalog:
                print("[!] Catalog is empty", flush=True)
                time.sleep(5)
                continue

            # Pick a random item
            selected_item = random.choice(catalog)
            item_id = selected_item.get("id", 101)
            qty = random.randint(1, 3)

            # 4. Save items to user's shopping cart
            print(f"-> Adding item {item_id} (qty: {qty}) to cart...", flush=True)
            cart_resp = requests.post(
                f"{GATEWAY_URL}/cart/{user_id}",
                headers=headers,
                json={"items": [{"item_id": item_id, "quantity": qty}]},
                timeout=5
            )
            if cart_resp.status_code != 200:
                print(f"[!] Cart update failed: {cart_resp.status_code}", flush=True)

            # 5. Retrieve cart to verify (simulate user shopping loop)
            requests.get(f"{GATEWAY_URL}/cart/{user_id}", headers=headers, timeout=5)

            # 6. Proceed to Checkout (Deep Cascade Cascade Route)
            print(f"-> Proceeding to checkout for Order: Item {item_id}...", flush=True)
            checkout_resp = requests.post(
                f"{GATEWAY_URL}/checkout",
                headers=headers,
                json={"item_id": item_id, "quantity": qty},
                timeout=5
            )
            if checkout_resp.status_code == 200:
                print(f"[+] Checkout completed successfully: {checkout_resp.json()}", flush=True)
            else:
                print(f"[!] Checkout failed: {checkout_resp.status_code} - {checkout_resp.text}", flush=True)

        except requests.exceptions.ConnectionError:
            print("[!] Gateway service unreachable, retrying in 5 seconds...", flush=True)
        except Exception as e:
            print(f"[!] Simulator encountered unexpected error: {e}", flush=True)
            
        # Wait before next simulation cycle
        sleep_duration = random.uniform(2, 6)
        time.sleep(sleep_duration)

if __name__ == "__main__":
    run_simulator()
