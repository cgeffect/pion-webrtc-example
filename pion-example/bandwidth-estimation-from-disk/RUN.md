
这段代码是一个使用 Pion WebRTC 实现的带宽估计（Bandwidth Estimation）示例，展示了如何根据网络带宽动态调整视频流的质量。它通过从磁盘读取 IVF 文件（一种简单的视频文件格式）来模拟视频流的发送，并根据带宽估计结果动态切换视频质量。

代码中的路径使用的是当前目录, 即```lowFile    = "low.ivf"```, 需要把这三个文件放到项目的根目录, 而不是当前main.go的目录


```
const (
    lowFile    = "low.ivf"
    lowBitrate = 300_000

    medFile    = "med.ivf"
    medBitrate = 1_000_000

    highFile    = "high.ivf"
    highBitrate = 2_500_000

    ivfHeaderSize = 32
)
```

启动main.go之后, 会看到过一会打印
```
Switching from low.ivf to med.ivf 
Switching from med.ivf to high.ivf 
```
表示切换的不同的分辨率, 在chrome浏览器里也可以看到, 在播放不同分辨率的视频

---
# 代码分析
当然可以！这段代码展示了如何使用 Pion WebRTC 实现一个基于带宽估计的动态视频质量调整系统。它从磁盘读取 IVF 文件，并根据网络带宽动态切换视频质量。以下是代码执行的详细流程梳理：

### 1. **初始化和配置**
#### 1.1 初始化质量级别
```go
qualityLevels := []struct {
    fileName string
    bitrate  int
}{
    {lowFile, lowBitrate},
    {medFile, medBitrate},
    {highFile, highBitrate},
}
currentQuality := 0
```
- 定义了三种视频质量级别（低、中、高），分别对应不同的 IVF 文件和比特率。
- `currentQuality` 用于跟踪当前使用的质量级别。

#### 1.2 检查文件是否存在
```go
for _, level := range qualityLevels {
    _, err := os.Stat(level.fileName)
    if os.IsNotExist(err) {
        panic(fmt.Sprintf("File %s was not found", level.fileName))
    }
}
```
- 遍历 `qualityLevels`，检查每个 IVF 文件是否存在。如果文件不存在，程序会 panic。

#### 1.3 初始化拦截器注册表和媒体引擎
```go
interceptorRegistry := &interceptor.Registry{}
mediaEngine := &webrtc.MediaEngine{}
if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
    panic(err)
}
```
- 创建拦截器注册表和媒体引擎。
- 注册默认的编解码器，确保 WebRTC 可以处理音视频流。

### 2. **创建带宽估计器**
```go
congestionController, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
    return gcc.NewSendSideBWE(gcc.SendSideBWEInitialBitrate(lowBitrate))
})
if err != nil {
    panic(err)
}
```
- 创建一个带宽估计器（Congestion Controller），使用 Google 拥塞控制算法（GCC）。
- `gcc.NewSendSideBWE` 初始化带宽估计器，初始比特率为 `lowBitrate`。

### 3. **注册带宽估计器回调**
```go
estimatorChan := make(chan cc.BandwidthEstimator, 1)
congestionController.OnNewPeerConnection(func(id string, estimator cc.BandwidthEstimator) {
    estimatorChan <- estimator
})
```
- 创建一个通道 `estimatorChan`，用于接收新创建的带宽估计器实例。
- 注册回调函数，当新的 `PeerConnection` 创建时，将带宽估计器实例发送到通道中。

### 4. **注册拦截器和配置 WebRTC**
```go
interceptorRegistry.Add(congestionController)
if err = webrtc.ConfigureTWCCHeaderExtensionSender(mediaEngine, interceptorRegistry); err != nil {
    panic(err)
}
if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
    panic(err)
}
```
- 将带宽估计器添加到拦截器注册表中。
- 配置传输时间戳（TWCC）扩展头。
- 注册默认的拦截器。

### 5. **创建 RTCPeerConnection**
```go
peerConnection, err := webrtc.NewAPI(
    webrtc.WithInterceptorRegistry(interceptorRegistry), webrtc.WithMediaEngine(mediaEngine),
).NewPeerConnection(webrtc.Configuration{
    ICEServers: []webrtc.ICEServer{
        {
            URLs: []string{"stun:stun.l.google.com:19302"},
        },
    },
})
if err != nil {
    panic(err)
}
defer func() {
    if cErr := peerConnection.Close(); cErr != nil {
        fmt.Printf("cannot close peerConnection: %v\n", cErr)
    }
}()
```
- 使用配置好的拦截器和媒体引擎创建一个新的 `RTCPeerConnection`。
- 配置了 STUN 服务器以支持 NAT 穿透。
- 使用 `defer` 确保在程序结束时关闭 `peerConnection`。

### 6. **等待带宽估计器创建完成**
```go
estimator := <-estimatorChan
```
- 从通道 `estimatorChan` 中接收带宽估计器实例。

### 7. **创建视频轨道**
```go
videoTrack, err := webrtc.NewTrackLocalStaticSample(
    webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion",
)
if err != nil {
    panic(err)
}
rtpSender, err := peerConnection.AddTrack(videoTrack)
if err != nil {
    panic(err)
}
```
- 创建一个本地视频轨道（`videoTrack`），用于发送视频数据。
- 将视频轨道添加到 `peerConnection` 中。

### 8. **读取 RTCP 数据**
```go
go func() {
    rtcpBuf := make([]byte, 1500)
    for {
        if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
            return
        }
    }
}()
```
- 启动一个 goroutine，读取 RTCP 数据。这些数据用于处理 NACK 等 RTCP 反馈。

### 9. **设置 ICE 和 PeerConnection 状态回调**
```go
peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
    fmt.Printf("Connection State has changed %s \n", connectionState.String())
})
peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
    fmt.Printf("Peer Connection State has changed: %s\n", state.String())
})
```
- 注册回调函数，用于处理 ICE 连接状态和 PeerConnection 状态的变化。

### 10. **处理信令**
```go
offer := webrtc.SessionDescription{}
decode(readUntilNewline(), &offer)
if err = peerConnection.SetRemoteDescription(offer); err != nil {
    panic(err)
}
```
- 从标准输入读取 Offer 并解码为 `SessionDescription`。
- 设置远端描述（Offer）。

### 11. **创建 Answer**
```go
answer, err := peerConnection.CreateAnswer(nil)
if err != nil {
    panic(err)
}
```
- 创建 Answer。

### 12. **等待 ICE 候选者收集完成**
```go
gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
if err = peerConnection.SetLocalDescription(answer); err != nil {
    panic(err)
}
<-gatherComplete
```
- 设置本地描述（Answer）。
- 等待 ICE 候选者收集完成。

### 13. **输出 Answer**
```go
fmt.Println(encode(peerConnection.LocalDescription()))
```
- 将 Answer 编码为 Base64 格式的 JSON 数据，并输出到标准输出。

### 14. **打开 IVF 文件并读取帧**
```go
file, err := os.Open(qualityLevels[currentQuality].fileName)
ivf, header, err := ivfreader.NewWith(file)
```
- 打开当前质量级别的 IVF 文件。
- 使用 `ivfreader` 读取文件头信息。

### 15. **动态调整视频质量**
```go
switchQualityLevel := func(newQualityLevel int) {
    fmt.Printf(
        "Switching from %s to %s \n",
        qualityLevels[currentQuality].fileName,
        qualityLevels[newQualityLevel].fileName,
    )
    currentQuality = newQualityLevel
    ivf.ResetReader(setReaderFile(qualityLevels[currentQuality].fileName))
    for {
        if frame, frameHeader, err = ivf.ParseNextFrame(); err != nil {
            break
        } else if frameHeader.Timestamp >= currentTimestamp && frame[0]&0x1 == 0 {
            break
        }
    }
}
```
- 根据带宽估计结果动态切换视频质量。
- 如果当前带宽低于当前质量的比特率，切换到较低质量。
- 如果当前带宽高于下一个质量的比特率，切换到较高质量。

### 16. **发送视频帧**
```go
ticker := time.NewTicker(
    time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000),
)
defer ticker.Stop()
for ; true; <-ticker.C {
    targetBitrate := estimator.GetTargetBitrate()
    switch {
    case currentQuality != 0 && targetBitrate < qualityLevels[currentQuality].bitrate:
        switchQualityLevel(currentQuality - 1)
    case len(qualityLevels) > (currentQuality+1) && targetBitrate > qualityLevels[currentQuality+1].bitrate:
        switchQualityLevel(currentQuality + 1)
    default:
        frame, frameHeader, err = ivf.ParseNextFrame()
    }

    switch {
    case errors.Is(err, io.EOF):
        ivf.ResetReader(setReaderFile(qualityLevels[currentQuality].fileName))
    case err == nil:
        currentTimestamp = frameHeader.Timestamp
        if err = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); err != nil {
            panic(err)
        }
    default:
        panic(err)
    }
}
```
- 使用 `time.Ticker` 按照视频帧的时间戳发送视频帧。
- 根据带宽估计结果动态调整视频质量。
- 如果到达文件末尾，则重新开始读取文件。

### 17. **辅助函数**
#### 17.1 打开 IVF 文件
```go
func setReaderFile(filename string) func(_ int64) io.Reader {
    return func(_ int64) io.Reader {
        file, err := os.Open(filename)
        if err != nil {
            panic(err)
        }
        if _, err = file.Seek(ivfHeaderSize, io.SeekStart); err != nil {
            panic(err)
        }
        return file
    }
}
```
- 用于打开 IVF 文件并跳过文件头。

#### 17.2 从标准输入读取一行
```go
func readUntilNewline() (in string) {
    var err error
    r := bufio.NewReader(os.Stdin)
    for {
        in, err = r.ReadString('\n')
        if err != nil && !errors.Is(err, io.EOF) {
            panic(err)
        }
        if in = strings.TrimSpace(in); len(in) > 0 {
            break
        }
    }
    fmt.Println("")
    return
}
```
- 从标准输入读取一行数据，直到遇到换行符。

#### 17.3 编码 SessionDescription
```go
func encode(obj *webrtc.SessionDescription) string {
    b, err := json.Marshal(obj)
    if err != nil {
        panic(err)
    }
    return base64.StdEncoding.EncodeToString(b)
}
```
- 将 `SessionDescription` 编码为 Base64 格式的 JSON 数据。

#### 17.4 解码 SessionDescription
```go
func decode(in string, obj *webrtc.SessionDescription) {
    b, err := base64.StdEncoding.DecodeString(in)
    if err != nil {
        panic(err)
    }
    if err = json.Unmarshal(b, obj); err != nil {
        panic(err)
    }
}
```
- 将 Base64 编码的 JSON 数据解码为 `SessionDescription`。

### 总结
这段代码展示了如何使用 Pion WebRTC 的带宽估计功能来动态调整视频流的质量。它从磁盘读取 IVF 文件，并根据网络带宽动态切换视频质量。这种技术在实时视频通信中非常重要，可以确保在不同网络条件下都能提供最佳的用户体验。


