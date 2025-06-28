package router

import (
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/api/request"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/api/response"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/pkg/ginx"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// UserController 用户方面接口
type UserController interface {
	UpdateUsers(g *gin.Context, req request.UpdateUserRoleReq, cla jwt.UserClaims) (response.Response, error)
	GetMyInfo(g *gin.Context, cla jwt.UserClaims) (response.Response, error)
	UpdateMyInfo(g *gin.Context, req request.UpdateUserReq, cla jwt.UserClaims) (response.Response, error)
	SelectUsers(g *gin.Context) (response.Response, error)
	GetUserInfo(g *gin.Context) (response.Response, error)
}

func UserRoutes(
	s *gin.RouterGroup,
	authMiddleware gin.HandlerFunc,
	c UserController,
) {
	//认证服务
	UserGroup := s.Group("/user")

	UserGroup.POST("/updateUser", authMiddleware, ginx.WrapClaimsAndReq(c.UpdateUsers))
	UserGroup.GET("/getMyInfo", authMiddleware, ginx.WrapClaims(c.GetMyInfo))
	UserGroup.POST("/updateMyInfo", authMiddleware, ginx.WrapClaimsAndReq(c.UpdateMyInfo))
	UserGroup.GET("/getUsers", authMiddleware, ginx.Wrap(c.SelectUsers))
	UserGroup.GET("/:id/getUserInfo", authMiddleware, ginx.Wrap(c.GetUserInfo))
}
