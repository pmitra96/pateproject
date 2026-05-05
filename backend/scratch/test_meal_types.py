import requests
import json
import time

BASE_URL = "https://pateproject-backend-hgh2pji4tq-as.a.run.app"
WHATSAPP_ENDPOINT = f"{BASE_URL}/whatsapp/webhook"
TEST_PHONE = "915555555555"

def send_message(text):
    print(f"\n[USER]: {text}")
    payload = {
        "object": "whatsapp_business_account",
        "entry": [{
            "id": "123456789",
            "changes": [{
                "value": {
                    "messaging_product": "whatsapp",
                    "metadata": {"display_phone_number": "12345", "phone_number_id": "12345"},
                    "contacts": [{"profile": {"name": "Test User"}, "wa_id": TEST_PHONE}],
                    "messages": [{
                        "from": TEST_PHONE,
                        "id": f"msg_{int(time.time())}",
                        "timestamp": str(int(time.time())),
                        "text": {"body": text},
                        "type": "text"
                    }]
                },
                "field": "messages"
            }]
        }]
    }
    
    try:
        response = requests.post(WHATSAPP_ENDPOINT, json=payload, timeout=15)
        print(f"[SERVER RESPONSE]: {response.status_code} - {response.text}")
    except Exception as e:
        print(f"[ERROR]: {e}")

test_cases = [
    "I had 1 bowl of oatmeal for breakfast.",
    "Actually, it was a large bowl of oatmeal with a sliced banana and honey.",
    "Show me my breakfast log."
]

print("=== STARTING MEAL TYPE TEST PLAN ===")
for test in test_cases:
    send_message(test)
    time.sleep(20)
print("\n=== TEST PLAN EXECUTION COMPLETE ===")
