使用命令行运行(直接GoLand运行好像不成功, 可以再尝试下)

PION_LOG_TRACE=all go run *.go

然后再chrome里打开多个http://localhost:8080/地址, 会看到多个画面
坑点:
在safair里只能打开一个tab, 打开多个tab的情况下, 不活跃的tab的摄像头会被关掉


运行flutter程序

cd sfu-ws/flutter
flutter create --project-name flutter_sfu_wsx_example --org com.github.pion .

flutter run

如果修改了flutter的工程目录再运行flutter run会出现路径不正确, 是因为Xcode编译缓存的问题, 先清空下缓存, flutter clean, 再运行flutter run

如果是自己手动打开Xcode工程运行, 需要添加
<key>NSCameraUsageDescription</key>
<string>$(PRODUCT_NAME) Camera Usage!</string>
<key>NSMicrophoneUsageDescription</key>
<string>$(PRODUCT_NAME) Microphone Usage!</string>
<key>NSAppTransportSecurity</key>
<dict>
    <key>NSAllowsArbitraryLoads</key>
    <true/>
</dict>

如果是使用flutter run运行则不需要添加