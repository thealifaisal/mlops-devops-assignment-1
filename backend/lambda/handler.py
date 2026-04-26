import json
import os
import urllib.request
import urllib.error


def lambda_handler(event, context):
    api_key = os.environ.get("OPENAI_API_KEY", "")
    if not api_key:
        result = {
            "text": "",
            "usage": {"inputTokens": 0, "outputTokens": 0},
            "error": "OPENAI_API_KEY is not set in Lambda environment"
        }
        _maybe_callback(event.get("callbackUrl", ""), result)
        return result

    prompt       = event.get("prompt", "")
    model        = event.get("model", "gpt-4o-mini")
    temperature  = event.get("temperature", 0.2)
    callback_url = event.get("callbackUrl", "")

    payload = json.dumps({
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": temperature
    }).encode("utf-8")

    req = urllib.request.Request(
        "https://api.openai.com/v1/chat/completions",
        data=payload,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json"
        },
        method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            body = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8", errors="replace")
        result = {
            "text": "",
            "usage": {"inputTokens": 0, "outputTokens": 0},
            "error": f"OpenAI HTTP {e.code}: {error_body}"
        }
        _maybe_callback(callback_url, result)
        return result
    except Exception as e:
        result = {
            "text": "",
            "usage": {"inputTokens": 0, "outputTokens": 0},
            "error": str(e)
        }
        _maybe_callback(callback_url, result)
        return result

    text  = body["choices"][0]["message"]["content"]
    usage = body.get("usage", {})

    result = {
        "text": text,
        "usage": {
            "inputTokens":  usage.get("prompt_tokens", 0),
            "outputTokens": usage.get("completion_tokens", 0)
        }
    }

    _maybe_callback(callback_url, result)
    return result


def _maybe_callback(callback_url, result):
    """POST result to the backend callback endpoint if a URL was provided."""
    if not callback_url:
        return
    try:
        data = json.dumps(result).encode("utf-8")
        req = urllib.request.Request(
            callback_url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST"
        )
        urllib.request.urlopen(req, timeout=10)
    except Exception as e:
        # Log but do not raise — the primary work (OpenAI call) already succeeded
        print(f"callback failed url={callback_url} error={e}")
