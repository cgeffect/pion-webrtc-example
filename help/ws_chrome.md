 Chrome 开发者工具中，你可以方便地测试和调试 WebSocket 连接。以下是如何使用 Chrome 开发者工具来测试 WebSocket 的步骤：

1. 打开 Chrome 开发者工具
右键点击页面，选择 “检查”（Inspect）。
或者，按 Ctrl+Shift+I（Windows/Linux）或 Cmd+Option+I（Mac）。
2. 测试 WebSocket
使用浏览器的控制台（Console）：
在开发者工具中切换到 “控制台”（Console）标签。
使用 JavaScript 创建一个 WebSocket 连接，并发送消息。例如：
JavaScript
复制
const ws = new WebSocket("ws://localhost:8000/ws");
ws.onopen = () => {
    console.log("WebSocket连接成功");
    ws.send("Hello, Server!");
};
ws.onmessage = (event) => {
    console.log("从服务器收到消息:", event.data);
};
ws.onclose = () => {
    console.log("WebSocket连接关闭");
};