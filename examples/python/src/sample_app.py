from http.server import BaseHTTPRequestHandler, HTTPServer
import socket
import requests
import httpx
import platform
import datetime
import urllib.request
import urllib.parse
import http.client
import urllib3

remote_url = "https://whoami.local.test"
version = platform.python_version()
hostname = socket.gethostname()


class SimpleHTTPRequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        response = "Hi, I'm Python/" + version + " service running on '" + hostname + "' host.\n\n"
        response += f"Time is {datetime.datetime.now().astimezone().isoformat(timespec='seconds')}\n\n"
        response += "Rendering " + remote_url + " page\n"

        fetch_methods_to_try = [
            ('requests', fetch_with_requests),
            ('httpx', fetch_with_httpx),
            ('urllib.request', fetch_with_urllib_request),
            ('urllib3', fetch_with_urllib3),
            ('http.client', fetch_with_http_client),
        ]

        for i, (name, method) in enumerate(fetch_methods_to_try):
            response += f"\nrequest ({i+1}) - using '{name}' lib:\n\n"

            try:
                response += method(remote_url) + "\n"
            except Exception as ex:
                response += indent_line(f"# Error: ({type(ex).__qualname__}) {ex}\n")

        response += "\nThank you!\n"

        self.send_response(200)
        self.send_header("Content-type", "text/plain")
        self.end_headers()
        self.wfile.write(response.encode("utf-8"))


def fetch_with_requests(remote_url: str) -> str:
    response = requests.get(remote_url)
    response.raise_for_status()
    return parse_content(response.text)


def fetch_with_httpx(remote_url: str) -> str:
    response = httpx.get(remote_url)
    response.raise_for_status()
    return parse_content(response.text)


def fetch_with_urllib_request(remote_url: str) -> str:
    request = urllib.request.Request(remote_url, method="GET")
    response = urllib.request.urlopen(request)
    return parse_content(response.read().decode())


def fetch_with_urllib3(remote_url: str) -> str:
    response = urllib3.request("GET", remote_url)
    if response.status >= 400:
        raise RuntimeError(f"HTTP Error {response.status}: {response.reason}")
    return parse_content(response.data.decode())


def fetch_with_http_client(remote_url: str) -> str:
    url = urllib.parse.urlparse(remote_url)
    conn = http.client.HTTPSConnection(str(url.hostname), url.port)
    conn.request("GET", url.path)
    response = conn.getresponse()
    if response.status >= 400:
        raise RuntimeError(f"HTTP Error {response.status}: {response.reason}")
    return parse_content(response.read().decode())


def parse_content(content: str) -> str:
    return "\n".join([
        indent_line(line)
        for line in content.split("\n")
        if expected_line(line)
    ])


def expected_line(line: str) -> bool:
    if "Hostname" in line:
        return True

    if "IP" in line and "." in line and "127.0.0." not in line:
        return True

    return False


def indent_line(line: str) -> str:
    return f"    {line}"


def run():
    httpd = HTTPServer(("", 8080), SimpleHTTPRequestHandler)
    print("Server running on http://localhost:8080/")
    httpd.serve_forever()


if __name__ == "__main__":
    run()
