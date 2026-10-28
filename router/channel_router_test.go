package router

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelStatusRoutesUseOperatePermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodPost, "/:id/status", authz.ChannelOperate, controller.UpdateChannelStatus)
	assertChannelRoutePermission(t, http.MethodPost, "/status/batch", authz.ChannelOperate, controller.BatchUpdateChannelStatus)
	assertChannelRoutePermission(t, http.MethodPut, "/", authz.ChannelWrite, controller.UpdateChannel)
}

func TestChannelDeleteRoutesUseSensitiveWritePermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodDelete, "/:id", authz.ChannelSensitiveWrite, controller.DeleteChannel)
	assertChannelRoutePermission(t, http.MethodPost, "/batch", authz.ChannelSensitiveWrite, controller.DeleteChannelBatch)
	assertChannelRoutePermission(t, http.MethodDelete, "/disabled", authz.ChannelSensitiveWrite, controller.DeleteDisabledChannel)
	assertChannelRoutePermission(t, http.MethodPut, "/", authz.ChannelWrite, controller.UpdateChannel)
	assertChannelRoutePermission(t, http.MethodPut, "/tag", authz.ChannelWrite, controller.EditTagChannels)
	assertChannelRoutePermission(t, http.MethodPost, "/batch/tag", authz.ChannelWrite, controller.BatchSetChannelTag)
}

func TestChannelStatusRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api")

	require.NotPanics(t, func() {
		registerChannelRoutes(api)
	})
}

// relay 的根级参数路由会让 gin 的尾斜杠重定向失效，前端调用不带斜杠的
// /api/channel 时必须直接命中路由，因此根路径要同时注册带斜杠和不带斜杠两个版本。
func TestChannelGroupRootRegistersBothSlashVariants(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, route := range channelPermissionRoutes {
		if route.path != "/" {
			continue
		}
		assert.True(t, registered[route.method+" /api/channel"], "%s /api/channel 未注册", route.method)
		assert.True(t, registered[route.method+" /api/channel/"], "%s /api/channel/ 未注册", route.method)
	}
}

// 回归：注册根路径别名时，handlers 必须同时包含权限中间件和业务 handler。
// 漏掉业务 handler 时路由依然注册成功（engine.Routes 里能看到），但请求只会
// 执行权限中间件并返回 200 空响应，前端表现为"操作失败"。
func TestChannelRootAliasKeepsBusinessHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))

	checked := 0
	for _, route := range channelPermissionRoutes {
		if route.path != "/" {
			continue
		}
		want := reflect.ValueOf(route.handler).Pointer()
		for _, info := range engine.Routes() {
			if info.Method != route.method {
				continue
			}
			if info.Path != "/api/channel" && info.Path != "/api/channel/" {
				continue
			}
			checked++
			require.NotNil(t, info.HandlerFunc, "%s %s 缺少 handler", info.Method, info.Path)
			assert.Equal(t, want, reflect.ValueOf(info.HandlerFunc).Pointer(),
				"%s %s 的最终 handler 不是业务 handler", info.Method, info.Path)
		}
	}
	assert.NotZero(t, checked, "未找到渠道根路径路由")
}

func assertChannelRoutePermission(t *testing.T, method string, path string, permission authz.Permission, handler any) {
	t.Helper()
	for _, route := range channelPermissionRoutes {
		if route.method == method && route.path == path {
			assert.Equal(t, permission, route.permission)
			assert.Equal(t, reflect.ValueOf(handler).Pointer(), reflect.ValueOf(route.handler).Pointer())
			return
		}
	}
	t.Fatalf("route %s %s not found", method, path)
}
