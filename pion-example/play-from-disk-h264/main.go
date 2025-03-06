// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// play-from-disk demonstrates how to send video and/or audio to your browser from files saved to disk.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/rtcp"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/h264reader"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

const (
	audioFileName     = "/Users/jason/Desktop/output.ogg"
	videoFileName     = "/Users/jason/Desktop/output.h264"
	oggPageDuration   = time.Millisecond * 20
	h264FrameDuration = time.Millisecond * 33
)

func main() { //nolint
	// Assert that we have an audio or video file
	_, err := os.Stat(videoFileName)
	haveVideoFile := !os.IsNotExist(err)

	_, err = os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile && !haveVideoFile {
		panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}

	// 定义 ICE 服务器的 URL
	var stunURL = "stun:stun.l.google.com:19302"
	// 创建 ICE 服务器配置
	iceServer := webrtc.ICEServer{URLs: []string{stunURL}}
	// 创建 ICE 服务器列表
	iceServers := []webrtc.ICEServer{iceServer}
	// 创建 WebRTC 配置
	config := webrtc.Configuration{ICEServers: iceServers}
	// 创建 PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	// 创建一个上下文iceConnectedCtx, 和取消上下文的函数iceConnectedCtxCancel
	// iceConnectedCtx可以类比wait, iceConnectedCtxCancel可以类比notify
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	if haveVideoFile {
		// Create a video track
		videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		rtpSender, videoTrackErr := peerConnection.AddTrack(videoTrack)
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		// 读取传入的RTCP数据包
		// 在这些分组返回之前，拦截器将对其进行处理。对于NACK之类的东西，需要调用this。
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				// 读取对端发送过来的rtcp包数据
				n, addr, rtcpErr := rtpSender.Read(rtcpBuf)
				if rtcpErr != nil {
					fmt.Printf("Error reading RTCP packet: %v\n", rtcpErr)
					return
				}

				// 遍历 interceptor.Attributes
				for key, value := range addr {
					fmt.Printf("key: %v\n", key)
					fmt.Printf("val: %v\n", value)
				}

				// 打印读取到的数据长度和来源地址
				fmt.Printf("Received RTCP packet from %v, length: %d bytes\n", addr, n)

				// 解析 RTCP 数据包
				packets, err := rtcp.Unmarshal(rtcpBuf[:n])
				if err != nil {
					fmt.Printf("Error unmarshalling RTCP packet: %v\n", err)
					continue
				}

				// 遍历解析后的 RTCP 数据包
				for _, packet := range packets {
					fmt.Printf("Received RTCP packet: %+v\n", packet)
					// 根据 RTCP 数据包类型处理内容
					switch p := packet.(type) {
					case *rtcp.SenderReport:
						fmt.Printf("SenderReport: %+v\n", p)
					case *rtcp.ReceiverReport:
						fmt.Printf("ReceiverReport: %+v\n", p)
					//case *rtcp.NackPair:
					//	fmt.Printf("Nack: %+v\n", p)
					case *rtcp.PictureLossIndication:
						fmt.Printf("PictureLossIndication: %+v\n", p)
					case *rtcp.FullIntraRequest:
						fmt.Printf("FullIntraRequest: %+v\n", p)
					default:
						fmt.Printf("Unknown RTCP packet type: %+v\n", p)
					}
				}
			}
		}()

		go func() { // 协程, 开启一个协程去执行任务
			// Open a H264 file and start reading using our IVFReader
			file, h264Err := os.Open(videoFileName)
			if h264Err != nil {
				panic(h264Err)
			}

			h264, h264Err := h264reader.NewReader(file)
			if h264Err != nil {
				panic(h264Err)
			}

			// Wait for connection established
			<-iceConnectedCtx.Done() // wait()

			// 定时器, 每隔h264FrameDuration时间回调一次时间
			ticker := time.NewTicker(h264FrameDuration)
			// 无限循环, 每隔h264FrameDuration时间回调一次时间, ticker.C会执行一次
			for ; true; <-ticker.C {
				nal, h264Err := h264.NextNAL()
				if errors.Is(h264Err, io.EOF) {
					fmt.Printf("All video frames parsed and sent")
					os.Exit(0) // 文件读取完成，退出程序
				}
				if h264Err != nil {
					panic(h264Err)
				}

				if h264Err = videoTrack.WriteSample(media.Sample{Data: nal.Data, Duration: h264FrameDuration}); h264Err != nil {
					panic(h264Err)
				}
			}
		}()
	}

	if haveAudioFile {
		// Create a audio track
		audioTrack, audioTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		rtpSender, audioTrackErr := peerConnection.AddTrack(audioTrack)
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		go func() {
			// Open a ogg file and start reading using our oggReader
			file, oggErr := os.Open(audioFileName)
			if oggErr != nil {
				panic(oggErr)
			}

			// Open on oggfile in non-checksum mode.
			ogg, _, oggErr := oggreader.NewWith(file)
			if oggErr != nil {
				panic(oggErr)
			}

			// Wait for connection established
			<-iceConnectedCtx.Done()

			// Keep track of last granule, the difference is the amount of samples in the buffer
			var lastGranule uint64

			// It is important to use a time.Ticker instead of time.Sleep because
			// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
			// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)
			ticker := time.NewTicker(oggPageDuration)
			for ; true; <-ticker.C {
				pageData, pageHeader, oggErr := ogg.ParseNextPage()
				if errors.Is(oggErr, io.EOF) {
					fmt.Printf("All audio pages parsed and sent")
					os.Exit(0)
				}

				if oggErr != nil {
					panic(oggErr)
				}

				// The amount of samples is the difference between the last and current timestamp
				sampleCount := float64(pageHeader.GranulePosition - lastGranule)
				lastGranule = pageHeader.GranulePosition
				sampleDuration := time.Duration((sampleCount/48000)*1000) * time.Millisecond

				if oggErr = audioTrack.WriteSample(media.Sample{Data: pageData, Duration: sampleDuration}); oggErr != nil {
					panic(oggErr)
				}
			}
		}()
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel() // notify()
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Wait for the offer to be pasted
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

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(encode(peerConnection.LocalDescription()))

	// Block forever
	select {}
}

// Read from stdin until we get a newline
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

// JSON encode + base64 a SessionDescription
func encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription
func decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}

func saveAsH264(data []byte) {
	// 示例：将数据存储到文件中
	err := ioutil.WriteFile("output.h264", data, 0644)
	if err != nil {
		log.Printf("Error saving as H.264: %v\n", err)
		return
	}
	log.Println("Data saved as H.264 format")
}
