#!/usr/bin/env python3
"""
Locust load test for Tokligence Gateway.

Benchmark methodology aligned with LiteLLM benchmarks:
- 1000 concurrent users
- 500 user/sec spawn rate
- 5 minute duration
- Loopback adapter (no external API calls)

Usage:
    # Web UI mode
    locust -f locustfile.py --host=http://localhost:8081

    # Headless mode
    locust -f locustfile.py \
        --host=http://localhost:8081 \
        --users=1000 \
        --spawn-rate=500 \
        --run-time=5m \
        --html=report.html \
        --csv=results
"""

import json
import time
import random
from locust import HttpUser, task, between, events


class GatewayUser(HttpUser):
    """
    Simulates a user making requests to the Tokligence Gateway.
    """

    # Wait time between requests (1-2 seconds)
    wait_time = between(1, 2)

    def on_start(self):
        """Called when a simulated user starts."""
        self.auth_token = "test"  # Auth disabled in benchmark mode
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.auth_token}"
        }

    @task(10)  # Weight: 10 (most common)
    def chat_completion_loopback(self):
        """
        Test basic chat completion with loopback adapter.
        This measures pure gateway overhead without external API latency.
        """
        payload = {
            "model": "loopback",
            "messages": [
                {"role": "user", "content": "Hello, how are you?"}
            ],
            "max_tokens": 100
        }

        with self.client.post(
            "/v1/chat/completions",
            json=payload,
            headers=self.headers,
            catch_response=True
        ) as response:
            if response.status_code == 200:
                try:
                    data = response.json()
                    if "choices" in data and len(data["choices"]) > 0:
                        response.success()
                    else:
                        response.failure(f"Invalid response structure: {data}")
                except json.JSONDecodeError:
                    response.failure("Invalid JSON response")
            else:
                response.failure(f"HTTP {response.status_code}: {response.text}")

    @task(3)  # Weight: 3
    def chat_completion_with_tools(self):
        """
        Test chat completion with tool calls.
        Measures overhead of tool call processing.
        """
        payload = {
            "model": "loopback",
            "messages": [
                {"role": "user", "content": "What's the weather in San Francisco?"}
            ],
            "tools": [
                {
                    "type": "function",
                    "function": {
                        "name": "get_weather",
                        "description": "Get weather for a location",
                        "parameters": {
                            "type": "object",
                            "properties": {
                                "location": {"type": "string"}
                            }
                        }
                    }
                }
            ],
            "max_tokens": 100
        }

        with self.client.post(
            "/v1/chat/completions",
            json=payload,
            headers=self.headers,
            catch_response=True
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"HTTP {response.status_code}")

    @task(2)  # Weight: 2
    def streaming_chat(self):
        """
        Test streaming chat completion.
        Measures SSE streaming overhead.
        """
        payload = {
            "model": "loopback",
            "messages": [
                {"role": "user", "content": "Tell me a short story"}
            ],
            "max_tokens": 200,
            "stream": True
        }

        start_time = time.time()

        with self.client.post(
            "/v1/chat/completions",
            json=payload,
            headers=self.headers,
            stream=True,
            catch_response=True
        ) as response:
            if response.status_code == 200:
                # Consume the stream
                chunk_count = 0
                for line in response.iter_lines():
                    if line:
                        chunk_count += 1

                total_time = int((time.time() - start_time) * 1000)

                # Record custom metric
                events.request.fire(
                    request_type="POST",
                    name="/v1/chat/completions (streaming)",
                    response_time=total_time,
                    response_length=chunk_count,
                    exception=None,
                    context={}
                )
                response.success()
            else:
                response.failure(f"HTTP {response.status_code}")

    @task(1)  # Weight: 1
    def health_check(self):
        """
        Test health check endpoint.
        Should be very fast.
        """
        with self.client.get("/health", catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"HTTP {response.status_code}")

    @task(1)  # Weight: 1
    def list_models(self):
        """
        Test model listing endpoint.
        """
        with self.client.get(
            "/v1/models",
            headers=self.headers,
            catch_response=True
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"HTTP {response.status_code}")


class HighThroughputUser(HttpUser):
    """
    High-throughput user for stress testing.
    No wait time between requests.
    """

    wait_time = between(0.1, 0.5)  # Very short wait

    def on_start(self):
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": "Bearer test"
        }

    @task
    def rapid_fire_requests(self):
        """Rapid-fire small requests."""
        payload = {
            "model": "loopback",
            "messages": [{"role": "user", "content": "hi"}],
            "max_tokens": 10
        }

        self.client.post(
            "/v1/chat/completions",
            json=payload,
            headers=self.headers
        )


@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    """Called when test starts."""
    print("\n" + "="*60)
    print("Tokligence Gateway Benchmark")
    print("="*60)
    print(f"Target: {environment.host}")
    print(f"Users: {environment.runner.target_user_count if hasattr(environment.runner, 'target_user_count') else 'N/A'}")
    print("="*60 + "\n")


@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    """Called when test stops."""
    stats = environment.stats

    print("\n" + "="*60)
    print("Benchmark Results Summary")
    print("="*60)

    if stats.total.num_requests > 0:
        print(f"Total Requests: {stats.total.num_requests:,}")
        print(f"Total Failures: {stats.total.num_failures:,}")
        print(f"Requests/sec: {stats.total.current_rps:.2f}")
        print(f"Median Latency: {stats.total.median_response_time:.0f} ms")
        print(f"P95 Latency: {stats.total.get_response_time_percentile(0.95):.0f} ms")
        print(f"P99 Latency: {stats.total.get_response_time_percentile(0.99):.0f} ms")
        print(f"Average Latency: {stats.total.avg_response_time:.2f} ms")
        print(f"Min Latency: {stats.total.min_response_time:.0f} ms")
        print(f"Max Latency: {stats.total.max_response_time:.0f} ms")

        if stats.total.num_requests > 0:
            error_rate = (stats.total.num_failures / stats.total.num_requests) * 100
            print(f"Error Rate: {error_rate:.2f}%")

    print("="*60 + "\n")
