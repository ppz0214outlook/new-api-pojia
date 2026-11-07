package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/pojia"

	"github.com/gin-gonic/gin"
)

// GetPojiaProfiles 返回内嵌破甲 profile 目录（不含 instruction 正文），
// 供渠道编辑界面渲染选择列表。
func GetPojiaProfiles(c *gin.Context) {
	common.ApiSuccess(c, pojia.Catalog())
}
