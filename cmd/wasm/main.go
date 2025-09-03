package main

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"

	"envoy-wasm-graphql-federation/pkg/filter"
)

// main 函数是 WASM 模块的入口点
func main() {
	// 设置 VM 上下文
	proxywasm.SetVMContext(&vmContext{})
}

// vmContext 实现 VMContext 接口
type vmContext struct {
	types.DefaultVMContext
}

// NewPluginContext 创建新的插件上下文
func (ctx *vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	return filter.NewRootContext(0)
}
