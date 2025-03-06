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