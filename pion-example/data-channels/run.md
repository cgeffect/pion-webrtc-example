在 WebRTC 的信令流程中，`SetRemoteDescription` 和 `SetLocalDescription` 是两个关键步骤，分别用于设置远端（对方）的 SDP（Session Description Protocol）和本地（自己）的 SDP。

### 1. `SetRemoteDescription`
```go
err = peerConnection.SetRemoteDescription(offer)
```
这行代码的作用是将远端（对方）发送过来的 SDP 设置为本地的远端描述。`offer` 是一个 `webrtc.SessionDescription` 类型的对象，包含了对方的媒体信息（如音视频编解码器、带宽、媒体类型等）和网络信息（如 ICE 候选者）。

- **`offer`**：通常是一个 SDP offer（提议），由对方生成并发送过来。它描述了对方希望建立的媒体会话的参数。
- **作用**：通过调用 `SetRemoteDescription`，本地的 `PeerConnection` 会解析对方的 SDP，并根据其内容准备本地的媒体处理逻辑（如编解码器协商）。

### 2. `SetLocalDescription`
```go
err = peerConnection.SetLocalDescription(answer)
```
这行代码的作用是将本地生成的 SDP 设置为本地描述。`answer` 是一个 `webrtc.SessionDescription` 类型的对象，包含了本地的媒体信息和网络信息。

- **`answer`**：通常是一个 SDP answer（应答），由本地生成并发送给对方。它描述了本地希望接受的媒体会话的参数。
- **作用**：通过调用 `SetLocalDescription`，本地的 `PeerConnection` 会将生成的 SDP 设置为本地描述，并准备好将这个 SDP 发送给对方，以便对方可以了解本地的媒体和网络配置。

### 信令流程概述
在 WebRTC 中，SDP 是用于描述媒体会话的协议，而信令流程是协商媒体会话参数的过程。以下是信令流程的简化步骤：
1. **创建 Offer**：
    - 一方（通常是发起方）创建一个 SDP offer。
    - 使用 `CreateOffer` 方法生成 offer，并通过 `SetLocalDescription` 设置为本地描述。
    - 将 offer 发送给对方。

2. **设置远端 Offer**：
    - 对方收到 offer 后，使用 `SetRemoteDescription` 设置为远端描述。

3. **创建 Answer**：
    - 对方创建一个 SDP answer，作为对 offer 的响应。
    - 使用 `CreateAnswer` 方法生成 answer，并通过 `SetLocalDescription` 设置为本地描述。
    - 将 answer 发送给发起方。

4. **设置远端 Answer**：
    - 发起方收到 answer 后，使用 `SetRemoteDescription` 设置为远端描述。

5. **交换 ICE 候选者**：
    - 在整个过程中，双方还会交换 ICE 候选者（网络地址信息），以便建立实际的媒体传输通道。

### 示例代码中的信令流程
在你的代码中：
- `SetRemoteDescription` 用于设置从对方接收到的 offer。
- `CreateAnswer` 用于生成本地的 answer。
- `SetLocalDescription` 用于设置本地的 answer。
- 最后，通过 `encode` 函数将 answer 转换为 Base64 编码的字符串，以便可以通过信令通道发送给对方。

### 总结
- **`SetRemoteDescription`**：设置远端的 SDP（offer 或 answer）。
- **`SetLocalDescription`**：设置本地的 SDP（offer 或 answer）。
- 这两个方法是 WebRTC 信令流程中的关键步骤，用于协商媒体会话的参数。

---

在这个示例中, 我们的连接建立的流程为, 先把浏览器的sdp给到main.go, 然后再把main.go的sdp给到浏览器, 双方建立连接之后就可以通信了, 注意这个示例没有主动交换候选者的过程. 是有ICE自动完成了

所有可以看到, 在web端的程序里是使用了createOffer创建sdp, 并把sdp设置给自己
```
pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)
```
然而在main.go里, 当拿到web端的sdp之后使用的是
```
answer, err := peerConnection.CreateAnswer(nil)
```
创建answer, 然后把sdp设置给自己
```
err = peerConnection.SetLocalDescription(answer)
```

---

为什么同一台电脑不需要主动交换ICE?

你的问题非常关键，它触及了 WebRTC 中 ICE（Interactive Connectivity Establishment）机制的核心逻辑。实际上，**WebRTC 并不知道客户端和服务器是否在同一台电脑上运行**。这个判断是由 ICE 机制通过一系列的连通性测试自动完成的，而不是由 STUN 服务器直接决定的。以下是详细的解释：

### 1. **WebRTC 和 ICE 的工作原理**
ICE 的目标是帮助两个 WebRTC 客户端（无论是浏览器还是原生应用）建立最优的点对点连接。它通过以下步骤实现：
1. **收集候选者**：
   - **主机候选者（Host Candidates）**：客户端直接从本地网络接口获取的地址。
   - **反射候选者（Server Reflexive Candidates）**：通过 STUN 服务器获取的地址。
   - **中继候选者（Relay Candidates）**：通过 TURN 服务器获取的地址。

2. **交换候选者**：
   - 通过信令服务器，客户端将收集到的候选者发送给对方。

3. **连通性测试**：
   - ICE 代理会尝试所有可能的候选者对（本地候选者与远端候选者的组合），通过发送 STUN Binding Requests 来测试连通性。
   - 如果某个候选者对的连通性测试成功，ICE 会记录下来。

4. **选择最优候选者对**：
   - ICE 会从所有成功的候选者对中选择最优的一个（通常是延迟最低的候选者对）来建立实际的媒体或数据传输通道。

### 2. **在同一台电脑上运行时的情况**
在同一台电脑上运行 WebRTC 客户端和服务器端时，网络环境非常简单，通常不需要复杂的 NAT 穿透。以下是 ICE 如何处理这种情况：

#### （1）**主机候选者的作用**
- **本地回环地址（`127.0.0.1`）**：如果客户端和服务器端都在同一台电脑上运行，它们会自动收集到本地回环地址（`127.0.0.1`）作为主机候选者。
- **连通性测试**：ICE 会尝试使用这些主机候选者进行连通性测试。由于它们都在同一台电脑上，测试会非常快速且成功。

#### （2）**STUN 服务器的作用**
- **反射候选者**：即使在同一台电脑上运行，STUN 服务器仍然会被调用来获取反射候选者。STUN 服务器会返回客户端的公网映射地址。
- **连通性测试**：ICE 也会尝试使用这些反射候选者进行连通性测试。然而，由于主机候选者（如 `127.0.0.1`）已经成功建立了连接，反射候选者通常不会被优先选择。

#### （3）**ICE 的选择逻辑**
- ICE 的目标是选择最优的候选者对。在同一台电脑上运行时，主机候选者（如 `127.0.0.1`）通常是延迟最低的候选者对。
- 即使 STUN 服务器返回了反射候选者，ICE 也会优先选择主机候选者，因为它们的连通性测试更快且更稳定。

### 3. **为什么不需要显式交换候选者**
在同一台电脑上运行时，客户端和服务器端可以不显式交换候选者，但仍然可以成功建立连接的原因是：
- **ICE 的默认行为**：ICE 会自动收集主机候选者，并尝试使用它们进行连通性测试。
- **简单的网络环境**：在同一台电脑上运行时，网络环境非常简单，主机候选者（如 `127.0.0.1`）通常可以直接建立连接。
- **STUN 服务器的辅助作用**：即使 STUN 服务器返回了反射候选者，这些候选者通常不会被优先选择，因为主机候选者的连通性测试已经成功。

### 4. **总结**
- **WebRTC 和 ICE 并不知道客户端和服务器是否在同一台电脑上运行**。它们只是按照 ICE 的标准流程收集候选者并进行连通性测试。
- **主机候选者**（如 `127.0.0.1`）在同一台电脑上运行时通常可以直接建立连接，因为它们的连通性测试非常快速且成功。
- **STUN 服务器**仍然会被调用来获取反射候选者，但这些候选者通常不会被优先选择，因为主机候选者已经足够。

因此，即使没有显式交换候选者，WebRTC 仍然可以成功建立连接，因为 ICE 机制会自动选择最优的候选者对来建立连接。