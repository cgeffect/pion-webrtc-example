// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// bandwidth-estimation-from-disk demonstrates how to use Pion's Bandwidth Estimation APIs.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/gcc"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	lowFile    = "low.ivf"
	lowBitrate = 300_000

	medFile    = "med.ivf"
	medBitrate = 1_000_000

	highFile    = "high.ivf"
	highBitrate = 2_500_000

	ivfHeaderSize = 32
)

func main() { //nolint:gocognit,cyclop,maintidx
	qualityLevels := []struct {
		fileName string
		bitrate  int
	}{
		{lowFile, lowBitrate},
		{medFile, medBitrate},
		{highFile, highBitrate},
	}
	currentQuality := 0

	for _, level := range qualityLevels {
		_, err := os.Stat(level.fileName)
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("File %s was not found", level.fileName))
		}
	}

	//1. 初始化和配置
	//interceptorRegistry := &interceptor.Registry{}：声明并初始化一个指向 interceptor.Registry 实例的指针。
	interceptorRegistry := &interceptor.Registry{}
	//MediaEngine 是 Pion WebRTC 中用于管理音视频编解码器的组件。它负责处理音视频流的编解码逻辑，包括选择合适的编解码器、配置编解码参数等。
	mediaEngine := &webrtc.MediaEngine{} //注册默认的编解码器
	//RegisterDefaultCodecs() 是 MediaEngine 的一个方法，用于注册默认的音视频编解码器。
	//Pion WebRTC 默认支持多种编解码器，例如 VP8、VP9、H264（视频）和 Opus（音频）。调用 RegisterDefaultCodecs() 方法会将这些默认编解码器注册到 mediaEngine 中。
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	//2.带宽估计器（Congestion Controller）
	//创建拥塞控制器。它会分析入站和出站数据，并就我们应该发送多少数据提供建议。
	//传递`nil`意味着我们使用默认的估计算法，即谷歌拥塞控制。
	//您可以使用Pion提供的其他方法，也可以自己编写！
	congestionController, err := cc.NewInterceptor(
		//这是一个匿名函数，它没有名字，但可以作为参数传递给其他函数。
		//这个匿名函数的返回类型是 (cc.BandwidthEstimator, error)，表示它返回一个 cc.BandwidthEstimator 实例和一个错误。
		func() (cc.BandwidthEstimator, error) {
			//调用 gcc.NewSendSideBWE 函数，创建了一个带宽估计器（Bandwidth Estimator）实例。
			//gcc.NewSendSideBWE 是 Pion 提供的一个函数，用于创建一个基于 Google 拥塞控制算法（GCC）的带宽估计器
			//gcc.SendSideBWEInitialBitrate(lowBitrate) 是一个配置函数，用于设置带宽估计器的初始比特率
			return gcc.NewSendSideBWE(gcc.SendSideBWEInitialBitrate(lowBitrate))
		},
	)
	//如果 cc.NewInterceptor 或 gcc.NewSendSideBWE 返回错误，程序会通过 panic 抛出异常并终止运行
	if err != nil {
		panic(err)
	}

	//3.注册拦截器
	estimatorChan := make(chan cc.BandwidthEstimator, 1)
	congestionController.OnNewPeerConnection(
		func(id string, estimator cc.BandwidthEstimator) { //nolint: revive
			estimatorChan <- estimator
		},
	)

	interceptorRegistry.Add(congestionController)
	if err = webrtc.ConfigureTWCCHeaderExtensionSender(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	//4. 创建 RTCPeerConnection
	peerConnection, err := webrtc.NewAPI(webrtc.WithInterceptorRegistry(interceptorRegistry), webrtc.WithMediaEngine(mediaEngine)).NewPeerConnection(webrtc.Configuration{
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

	// Wait until our Bandwidth Estimator has been created
	estimator := <-estimatorChan

	// 创建一个本地视频轨道（videoTrack），用于发送视频数据。
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		panic(err)
	}

	// 将视频轨道添加到 peerConnection 中。
	rtpSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}

	//读取传入的RTCP数据包
	//在这些数据包返回之前，它们由拦截器处理。对于NACK之类的东西，需要调用this。
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())
	})

	//5. 处理信令 Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	decode(readUntilNewline(), &offer)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// 阻塞直到 ICE 收集完成，
	// 禁用trickle ICE 我们这样做是因为我们只能在生产应用程序中交换一个信令消息你应该通过 OnICECandidate 交换 ICE 候选
	// <-gatherComplete 是一个通道（channel）操作，表示从通道 gatherComplete 中接收数据。
	// 如果通道中有数据可接收，它会将数据取出；如果没有数据，它会阻塞，直到有数据可用
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(encode(peerConnection.LocalDescription()))

	// Open a IVF file and start reading using our IVFReader
	file, err := os.Open(qualityLevels[currentQuality].fileName)
	if err != nil {
		panic(err)
	}

	ivf, header, err := ivfreader.NewWith(file)
	if err != nil {
		panic(err)
	}

	//每次发送我们的视频文件帧。调整发送的速度，以便我们以与播放速度相同的速度发送。
	//这不是必需的，因为视频是有时间戳的，但是如果我们一次发送所有内容，我们将会有更高的损失。
	//
	//重要的是要利用时间。用时钟代替时间。睡眠是因为*避免累积歪斜，只是调用时间。
	//睡眠并不能弥补解析数据所花费的时间*可以解决睡眠的延迟问题（参见https://github.com/golang/go/issues/44343）

	//使用 time.Ticker 按照视频帧的时间戳发送视频帧。
	//根据带宽估计结果动态调整视频质量。
	//如果到达文件末尾，则重新开始读取文件。
	ticker := time.NewTicker(
		time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000),
	)
	defer ticker.Stop()
	frame := []byte{}
	frameHeader := &ivfreader.IVFFrameHeader{}
	currentTimestamp := uint64(0)

	// 动态调整视频质量
	//根据带宽估计结果动态切换视频质量。如果当前带宽低于当前质量的比特率，则切换到较低质量；如果高于下一个质量的比特率，则切换到较高质量。
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

	for ; true; <-ticker.C {
		targetBitrate := estimator.GetTargetBitrate()
		switch {
		// If current quality level is below target bitrate drop to level below
		case currentQuality != 0 && targetBitrate < qualityLevels[currentQuality].bitrate:
			switchQualityLevel(currentQuality - 1)

			// If next quality level is above target bitrate move to next level
		case len(qualityLevels) > (currentQuality+1) && targetBitrate > qualityLevels[currentQuality+1].bitrate:
			switchQualityLevel(currentQuality + 1)

		// Adjust outbound bandwidth for probing
		default:
			frame, frameHeader, err = ivf.ParseNextFrame()
		}

		switch {
		// If we have reached the end of the file start again
		case errors.Is(err, io.EOF):
			ivf.ResetReader(setReaderFile(qualityLevels[currentQuality].fileName))

		// No error write the video frame
		case err == nil:
			currentTimestamp = frameHeader.Timestamp
			if err = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); err != nil {
				panic(err)
			}
		// Error besides io.EOF that we dont know how to handle
		default:
			panic(err)
		}
	}
}

// 用于打开 IVF 文件并跳过文件头
func setReaderFile(filename string) func(_ int64) io.Reader {
	return func(_ int64) io.Reader {
		file, err := os.Open(filename) // nolint
		if err != nil {
			panic(err)
		}
		if _, err = file.Seek(ivfHeaderSize, io.SeekStart); err != nil {
			panic(err)
		}

		return file
	}
}

// Read from stdin until we get a newline.
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

// JSON encode + base64 a SessionDescription.
func encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription.
func decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}
