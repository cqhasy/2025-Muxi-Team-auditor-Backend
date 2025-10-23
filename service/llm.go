package service

import (
	"context"
	"errors"
	"time"

	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/api/request"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/api/response"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/langchain/client"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/langchain/model"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/langchain/prompt"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/pkg/logger"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/repository/cache"
	"github.com/cqhasy/2025-Muxi-Team-auditor-Backend/repository/dao"
	"github.com/redis/go-redis/v9"
)

const (
	maxJobs    = 100
	maxResults = 100
	retry      = 3
)

type LLMService struct {
	log     logger.Logger
	userDAO dao.UserDAOInterface
	itemDAO dao.ItemDaoInterface
	proDAO  dao.ProjectDAOInterface
	pcache  cache.ProjectCacheInterface
	client  client.AuditAIClient
	jobs    chan request.AuditItem
	results chan model.AuditResult
}

func NewLLMService(userDAO *dao.UserDAO, itemDAO *dao.ItemDao, proDAO *dao.ProjectDAO,
	c client.AuditAIClient, lo logger.Logger, pc *cache.ProjectCache) *LLMService {
	l := LLMService{
		userDAO: userDAO,
		itemDAO: itemDAO,
		proDAO:  proDAO,
		log:     lo,
		client:  c,
		jobs:    make(chan request.AuditItem, maxJobs),
		results: make(chan model.AuditResult, maxResults),
		pcache:  pc,
	}
	go l.worker()
	go l.consumer()
	return &l
}

func (l *LLMService) worker() {
	for item := range l.jobs {
		role, err := l.pcache.GetAuditRole(context.Background(), item.ProjectID)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				role, err = l.proDAO.GetProjectRole(context.Background(), item.ProjectID)
				if err != nil {
					l.log.Error(err.Error())
					continue
				}
				go func(pid uint, role string) {
					if err = l.pcache.SetAuditRole(context.Background(), pid, role); err != nil {
						l.log.Error(err.Error())
					}
				}(item.ProjectID, role)
			}
			l.log.Error(err.Error())
		}
		l.sendToLLM(item.ID, role, item.Contents)
	}
}

func (l *LLMService) consumer() {
	for result := range l.results {
		// 首次记录ai审核结果:待确认状态
		err := l.itemDAO.AuditItem(result.ID, result.Result, result.Reason)
		if err != nil {
			l.log.Error(err.Error())
			continue
		}
		l.log.Info("ai audit result before hook", logger.Int("ItemID", int(result.ID)),
			logger.String("Reason", result.Reason),
			logger.String("Result", auditStatusToString(result.Result)),
			logger.Float32("confidence", result.Confidence))

		// 尝试hook审核结果
		var f bool = false
		for i := 0; i < retry; i++ {
			item, err := l.itemDAO.FindItemByID(context.Background(), result.ID)
			if err != nil {
				l.log.Error(err.Error())
				continue
			}
			var data = request.WebHookData{
				Id:     item.HookId,
				Status: auditStatusForHook(item.Status),
				Msg:    item.Reason,
			}
			_, err = hookBack(item.HookUrl, request.HookPayload{
				Event: "audit result back",
				Data:  data,
				Try:   retry,
			}, "")
			if err != nil {
				l.log.Error(err.Error())
				break
			}
			f = true
			break
		}

		// 兜底，如果仍为false,则回滚为pending
		if f == false {
			err := l.itemDAO.AuditItem(result.ID, Pending, "")
			if err != nil {
				l.log.Error(err.Error())
			}
			continue
		}
		// 成功记录日志
		l.log.Info("ai audit result after hook", logger.Int("ItemID", int(result.ID)),
			logger.String("Result", auditStatusToString(result.Result)),
			logger.Float32("confidence", result.Confidence))
	}
}

func (l *LLMService) sendToLLM(id uint, r string, c response.Contents) {
	for retry := 0; retry < 3; retry++ {
		resp, err := l.client.SendMessage(prompt.BuildPrompt(r, c))
		if err == nil {
			resp.ID = id
			if resp.Confidence >= 0.5 {
				l.results <- resp
			}
			return
		}
		l.log.Warn("retrying sendToLLM", logger.Int("retry", retry), logger.Error(err))
		time.Sleep(time.Second * time.Duration(retry+1))
	}
	l.log.Error("sendToLLM failed after retries", logger.Int("ItemID", int(id)))
}

func (l *LLMService) Audit(Data []request.AuditItem) {
	for _, item := range Data {
		l.jobs <- item
	}
}
