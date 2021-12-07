package egroup

import (
	"context"
	"os"
	"syscall"
	"time"
)

var admin *LifeAdmin

func mockStart() func(ctx context.Context) error {
	return nil
}

func mockShutdown() func(ctx context.Context) error {
	return nil
}

type mockLifeAdminer struct{}

func (m *mockLifeAdminer) Start(ctx context.Context) error {
	return nil
}

func (m *mockLifeAdminer) Shutdown(ctx context.Context) error {
	return nil
}

func ExampleNewLifeAdmin() {
	// 默认配置
	// admin = NewLifeAdmin()

	// 可供选择配置选项

	// 设置启动超时时间
	// <=0 不启动超时时间,注意要在shutdown处理关闭通知
	// SetStartTimeout(time.Second)

	//  设置关闭超时时间
	//	<=0 不启动超时时间
	// SetStopTimeout(time.Second)

	// 设置信号集合,和处理信号的函数
	//SetSignal(func(lifeAdmin *LifeAdmin, signal os.Signal) {
	//	return
	//}, signal...)

	admin = NewLifeAdmin(SetStartTimeout(time.Second), SetStopTimeout(time.Second), SetSignal(func(a *LifeAdmin, signal os.Signal) {
		switch signal {
		case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
			a.shutdown()
		default:
		}
	}))
}

func ExampleLifeAdmin_Add() {
	// 通过struct添加
	admin.Add(Member{
		Start:    mockStart(),
		Shutdown: mockShutdown(),
	})
}

func ExampleLifeAdmin_AddMember() {
	// 根据接口适配添加
	admin.AddMember(&mockLifeAdminer{})
}

func ExampleLifeAdmin_Start() {
	defer admin.Shutdown()
	if err := admin.Start(); err != nil {
		// 处理错误
		// 正常启动会hold主
	}
}
