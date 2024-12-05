import time
import os
import requests
# import clipboard
import pyperclip
import time
import websocket
import json
from hashlib import sha256

# 服务器地址
SERVER_URL = "http://10.184.102.54:5000"
UPLOAD_ENDPOINT = f"{SERVER_URL}/upload/file"
TEXT_ENDPOINT = f"{SERVER_URL}/upload/text"
WEBSOCKET_ENDPOINT = f"ws://10.184.102.54:5000/ws"

# 用于记录上一次的剪切板内容
last_clipboard_content = ""


def monitor_clipboard():
       global last_clipboard_content
       while True:
           try:
               current_content = pyperclip.paste()
               if current_content != last_clipboard_content:
                   print(f"Clipboard updated: {current_content}")
                   last_clipboard_content = current_content
                   if is_valid_file(current_content):  # 如果剪切板内容是文件路径
                       upload_file_to_server(current_content)
                   else:  # 普通文本
                       upload_text_to_server(current_content)
               time.sleep(1)
           except Exception as e:
               print(f"Error checking pyperclip clipboard: {e}")
               time.sleep(5)

def is_valid_file(file_path):
    return os.path.isfile(file_path) and os.access(file_path, os.R_OK)

# 计算文件的 SHA256 哈希值
def calculate_file_hash(file_path):
    hash_func = sha256()
    with open(file_path, "rb") as f:
        while chunk := f.read(4096):
            hash_func.update(chunk)
    return hash_func.hexdigest()

# 上传文本内容到服务器
def upload_text_to_server(text):
    try:
        print(f"Uploading text to server:\n{text[:20] + '...' if len(text) > 20 else text}")
        response = requests.post(
            TEXT_ENDPOINT,
            data={"content": text, "type": "text"},
        )
        if response.status_code == 200:
            print(f"Text uploaded successfully: {text}")
        else:
            print(f"Failed to upload text: {response.json()}")
    except Exception as e:
        print(f"Error uploading text: {e}")

# 上传文件到服务器
def upload_file_to_server(file_path):
    try:
        print(f"Uploading file to server: {file_path}")
        file_hash = calculate_file_hash(file_path)
        with open(file_path, "rb") as file:
            files = {"file": (os.path.basename(file_path), file)}
            response = requests.post(
                UPLOAD_ENDPOINT,
                files=files,
                data={"hash": file_hash},
            )
        if response.status_code == 200:
            print(f"File uploaded successfully: {file_path}")
        else:
            print(f"Failed to upload file: {response.json()}")
    except Exception as e:
        print(f"Error uploading file: {e}")

# 检查剪切板变化
def check_clipboard_changes():
    global last_clipboard_content
    while True:
        try:
            current_content = clipboard.paste()
            if current_content != last_clipboard_content:
                print(f"Clipboard updated: {current_content}")
                last_clipboard_content = current_content
                if os.path.isfile(current_content):  # 如果剪切板内容是文件路径
                    upload_file_to_server(current_content)
                else:  # 普通文本
                    upload_text_to_server(current_content)
            time.sleep(1)
        except Exception as e:
            print(f"Error checking clipboard: {e}")
            time.sleep(5)

# WebSocket 事件处理
def on_message(ws, message):
    data = json.loads(message)
    if data["status"] == "uploading":
        print(f"Upload Progress: {data['progress']:.2f}%")
    elif data["status"] == "completed":
        print(f"Upload Completed: {data['path']}")

def on_error(ws, error):
    print(f"WebSocket Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("WebSocket connection closed")

def on_open(ws):
    print("WebSocket connection established")

# 启动 WebSocket 客户端
def start_websocket_client():
    while True:
        try:
            ws = websocket.WebSocketApp(
                WEBSOCKET_ENDPOINT,
                on_message=on_message,
                on_error=on_error,
                on_close=on_close,
            )
            ws.on_open = on_open
            ws.run_forever()
        except Exception as e:
            print(f"WebSocket connection error: {e}, retrying in 5 seconds...")
            time.sleep(5)

if __name__ == "__main__":
    from threading import Thread

    # 启动 WebSocket 客户端
    websocket_thread = Thread(target=start_websocket_client, daemon=True)
    websocket_thread.start()

    # 监听剪切板变化
    print("Starting clipboard monitoring...")
    # check_clipboard_changes()
    monitor_clipboard()
