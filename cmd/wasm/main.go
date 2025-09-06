package main

import (
	"envoy-wasm-graphql-federation/pkg/filter"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

// vmContext 实现 VMContext 接口
type vmContext struct {
	types.DefaultVMContext
}

// NewPluginContext 创建新的插件上下文
func (ctx *vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	return filter.NewRootContext(0)
}

func main() {}
func init() {
	proxywasm.SetVMContext(&vmContext{})
	// Plugin authors can use any one of four entrypoints, such as
	// `proxywasm.SetVMContext`, `proxywasm.SetPluginContext`, or
	// `proxywasm.SetTcpContext`.
	// proxywasm.SetHttpContext(func(contextID uint32) types.HttpContext {
	// 	return &httpContext{}
	// })
}