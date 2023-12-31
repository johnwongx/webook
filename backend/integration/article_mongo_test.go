package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/johnwongx/webook/backend/integration/startup"
	"github.com/johnwongx/webook/backend/internal/domain"
	"github.com/johnwongx/webook/backend/internal/repository/dao/article"
	myjwt "github.com/johnwongx/webook/backend/internal/web/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type ArticleMongoHandlerTestSuite struct {
	suite.Suite
	s       *gin.Engine
	col     *mongo.Collection
	liveCol *mongo.Collection
}

func (s *ArticleMongoHandlerTestSuite) SetupSuite() {
	s.s = gin.Default()
	mdb := startup.InitTestMongoDB()
	err := article.InitCollections(mdb)
	assert.NoError(s.T(), err)
	s.col = mdb.Collection("articles")
	if s.col == nil {
		panic("collection is nil")
	}
	s.liveCol = mdb.Collection("published_articles")
	if s.liveCol == nil {
		panic("collection is nil")
	}
	s.s.Use(func(ctx *gin.Context) {
		ctx.Set("claims", myjwt.UserClaim{
			UserId: 123,
		})
		ctx.Next()
	})
	node, err := snowflake.NewNode(1)
	assert.NoError(s.T(), err)
	hdl := startup.InitArticleHandler(article.NewMongoArticleDAO(mdb, node))
	hdl.RegisterRutes(s.s)
}

func (s *ArticleMongoHandlerTestSuite) TearDownTest() {
	_, err := s.col.DeleteMany(context.Background(), bson.M{})
	if err != nil {
		panic(fmt.Errorf("清空article表失败, 原因 %w", err))
	}
	_, err = s.liveCol.DeleteMany(context.Background(), bson.M{})
	if err != nil {
		panic(fmt.Errorf("清空published article表失败, 原因 %w", err))
	}
}

func TestArticleMongo(t *testing.T) {
	suite.Run(t, new(ArticleMongoHandlerTestSuite))
}

func (s *ArticleMongoHandlerTestSuite) TestArticleHandler_Withdraw() {
	testCases := []struct {
		name     string
		before   func(t *testing.T)
		after    func(t *testing.T)
		req      string
		wantCode int
		wantRes  Result[int64]
	}{
		{
			name: "edit my article",
			before: func(t *testing.T) {
				art := article.Article{
					Id:       3,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 123,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				_, err := s.col.InsertOne(ctx, art)
				assert.NoError(t, err)
				_, err = s.liveCol.InsertOne(ctx, article.PublishArticle(art))
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				//检查数据库中是否有对应数据
				var eArt article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 3}).Decode(&eArt)
				assert.NoError(t, err)
				assert.True(t, eArt.Utime > 678)
				eArt.Utime = 0
				assert.Equal(t, article.Article{
					Id:       3,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 123,
					Ctime:    123,
					Status:   domain.ArticleStatusPrivate.ToUint8(),
				}, eArt)

				var art article.PublishArticle
				err = s.liveCol.FindOne(ctx, bson.M{"id": 3}).Decode(&art)
				assert.NoError(t, err)
				assert.True(t, art.Utime > 678)
				art.Utime = 0
				assert.Equal(t, article.PublishArticle{
					Id:       3,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 123,
					Ctime:    123,
					Status:   domain.ArticleStatusPrivate.ToUint8(),
				}, art)
			},
			req: `
{
	"id":3
}
`,
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 3,
			},
		},
		{
			name: "edit others article, fail",
			before: func(t *testing.T) {
				art := article.Article{
					Id:       4,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				_, err := s.col.InsertOne(ctx, art)
				assert.NoError(t, err)
				_, err = s.liveCol.InsertOne(ctx, article.PublishArticle(art))
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				//检查数据库中是否有对应数据
				var eArt article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 4}).Decode(&eArt)
				assert.NoError(t, err)
				assert.Equal(t, article.Article{
					Id:       4,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, eArt)

				var art article.PublishArticle
				err = s.liveCol.FindOne(ctx, bson.M{"id": 4}).Decode(&art)
				assert.NoError(t, err)
				assert.Equal(t, article.PublishArticle{
					Id:       4,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, art)
			},
			req: `
{
	"id":4
}`,
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Code: 5,
				Msg:  "系统错误",
			},
		},
		{
			name:   "数据格式错误",
			before: func(t *testing.T) {},
			after:  func(t *testing.T) {},
			req: `
{
	"id":
}`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)

			req, err := http.NewRequest(http.MethodPost, "/articles/withdraw", bytes.NewReader([]byte(tc.req)))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp := httptest.NewRecorder()
			s.s.ServeHTTP(resp, req)
			assert.Equal(t, tc.wantCode, resp.Code)
			if resp.Code != http.StatusOK {
				return
			}

			var res Result[int64]
			err = json.NewDecoder(resp.Body).Decode(&res)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRes, res)

			tc.after(t)
		})
	}
}

func (s *ArticleMongoHandlerTestSuite) TestArticleHandler_Publish() {
	testCases := []struct {
		name     string
		before   func(t *testing.T)
		after    func(t *testing.T)
		Article  Article
		wantCode int
		wantRes  Result[int64]
	}{
		{
			name:   "create new article, publish success",
			before: func(t *testing.T) {},
			after: func(t *testing.T) {
				//检查数据库中是否有对应数据
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				var art article.PublishArticle
				err := s.liveCol.FindOne(ctx, bson.M{"author_id": 123}).Decode(&art)
				assert.NoError(t, err)
				assert.True(t, art.Ctime > 0)
				assert.True(t, art.Utime > 0)
				assert.True(t, art.Id > 0)
				art.Ctime = 0
				art.Utime = 0
				art.Id = 0
				assert.Equal(t, article.PublishArticle{
					Title:    "A Title",
					Content:  "This is content",
					AuthorId: 123,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Title:   "A Title",
				Content: "This is content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 1,
			},
		},
		{
			name: "update article,publish repo not existed",
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				art := article.Article{
					Id:       2,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 123,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusUnpublished.ToUint8(),
				}
				_, err := s.col.InsertOne(ctx, art)
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				//检查数据库中是否有对应数据
				var eArt article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 2}).Decode(&eArt)
				assert.NoError(t, err)
				assert.True(t, eArt.Utime > 678)
				eArt.Utime = 0
				assert.Equal(t, article.Article{
					Id:       2,
					Title:    "New Title",
					Content:  "new content",
					AuthorId: 123,
					Ctime:    123,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, eArt)

				var art article.PublishArticle
				err = s.liveCol.FindOne(ctx, bson.M{"id": 2}).Decode(&art)
				assert.NoError(t, err)
				assert.True(t, art.Utime > 678)
				assert.True(t, art.Ctime > 123)
				art.Utime = 0
				art.Ctime = 0
				assert.Equal(t, article.PublishArticle{
					Id:       2,
					Title:    "New Title",
					Content:  "new content",
					AuthorId: 123,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Id:      2,
				Title:   "New Title",
				Content: "new content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 2,
			},
		},
		{
			name: "update article, both repo existed",
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				art := article.Article{
					Id:       3,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 123,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}
				_, err := s.col.InsertOne(ctx, art)
				assert.NoError(t, err)
				_, err = s.liveCol.InsertOne(ctx, article.PublishArticle(art))
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				//检查数据库中是否有对应数据
				var eArt article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 3}).Decode(&eArt)
				assert.NoError(t, err)
				assert.True(t, eArt.Utime > 678)
				eArt.Utime = 0
				assert.Equal(t, article.Article{
					Id:       3,
					Title:    "New Title",
					Content:  "new content",
					AuthorId: 123,
					Ctime:    123,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, eArt)

				var art article.PublishArticle
				err = s.liveCol.FindOne(ctx, bson.M{"id": 3}).Decode(&art)
				assert.NoError(t, err)
				assert.True(t, art.Utime > 678)
				art.Utime = 0
				assert.Equal(t, article.PublishArticle{
					Id:       3,
					Title:    "New Title",
					Content:  "new content",
					AuthorId: 123,
					Ctime:    123,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Id:      3,
				Title:   "New Title",
				Content: "new content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 3,
			},
		},
		{
			name: "edit others article, fail",
			before: func(t *testing.T) {
				art := article.Article{
					Id:       4,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				_, err := s.col.InsertOne(ctx, art)
				assert.NoError(t, err)
				_, err = s.liveCol.InsertOne(ctx, article.PublishArticle(art))
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				//检查数据库中是否有对应数据
				var eArt article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 4}).Decode(&eArt)
				assert.NoError(t, err)
				assert.Equal(t, article.Article{
					Id:       4,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, eArt)

				var art article.PublishArticle
				err = s.liveCol.FindOne(ctx, bson.M{"id": 4}).Decode(&art)
				assert.NoError(t, err)
				assert.Equal(t, article.PublishArticle{
					Id:       4,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Id:      4,
				Title:   "New Title",
				Content: "new content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Code: 5,
				Msg:  "系统错误",
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)

			data, err := json.Marshal(tc.Article)
			assert.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, "/articles/publish", bytes.NewReader(data))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp := httptest.NewRecorder()
			s.s.ServeHTTP(resp, req)
			assert.Equal(t, tc.wantCode, resp.Code)
			if resp.Code != http.StatusOK {
				return
			}

			var res Result[int64]
			err = json.NewDecoder(resp.Body).Decode(&res)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRes.Code, res.Code)
			if tc.wantRes.Data > 0 {
				assert.True(t, res.Data > 0)
			}

			tc.after(t)
		})
	}
}

func (s *ArticleMongoHandlerTestSuite) TestArticleHandler_Edit() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	testCases := []struct {
		name     string
		before   func(t *testing.T)
		after    func(t *testing.T)
		Article  Article
		wantCode int
		wantRes  Result[int64]
	}{
		{
			name:   "new article edit success",
			before: func(t *testing.T) {},
			after: func(t *testing.T) {
				//检查数据库中是否有对应数据
				var art article.Article
				err := s.col.FindOne(ctx, bson.M{"author_id": 123}).Decode(&art)
				assert.NoError(t, err)
				assert.True(t, art.Id > 0)
				assert.True(t, art.Ctime > 0)
				assert.True(t, art.Utime > 0)
				art.Id = 0
				art.Ctime = 0
				art.Utime = 0
				assert.Equal(t, article.Article{
					Title:    "A Title",
					Content:  "This is content",
					AuthorId: 123,
					Status:   domain.ArticleStatusUnpublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Title:   "A Title",
				Content: "This is content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 1,
			},
		},
		{
			name: "edit existed article success",
			before: func(t *testing.T) {
				res, err := s.col.InsertOne(ctx, article.Article{
					Id:       2,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 123,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				})
				assert.NotNil(t, res)
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				//检查数据库中是否有对应数据
				var art article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 2}).Decode(&art)
				assert.NoError(t, err)
				assert.True(t, art.Utime > 678)
				art.Utime = 0
				assert.Equal(t, article.Article{
					Id:       2,
					Title:    "New Title",
					Content:  "new content",
					AuthorId: 123,
					Ctime:    123,
					Status:   domain.ArticleStatusUnpublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Id:      2,
				Title:   "New Title",
				Content: "new content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 2,
			},
		},
		{
			name: "edit others article",
			before: func(t *testing.T) {
				res, err := s.col.InsertOne(ctx, article.Article{
					Id:       3,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				})
				assert.NotNil(t, res)
				assert.NoError(t, err)
			},
			after: func(t *testing.T) {
				//检查数据库中是否有对应数据
				var art article.Article
				err := s.col.FindOne(ctx, bson.M{"id": 3}).Decode(&art)
				assert.NoError(t, err)
				assert.Equal(t, article.Article{
					Id:       3,
					Title:    "My tittle",
					Content:  "My Content",
					AuthorId: 233,
					Ctime:    123,
					Utime:    678,
					Status:   domain.ArticleStatusPublished.ToUint8(),
				}, art)
			},
			Article: Article{
				Id:      3,
				Title:   "New Title",
				Content: "new content",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Code: 5,
				Msg:  "系统错误",
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)

			data, err := json.Marshal(tc.Article)
			assert.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, "/articles/edit", bytes.NewReader(data))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp := httptest.NewRecorder()
			s.s.ServeHTTP(resp, req)
			assert.Equal(t, tc.wantCode, resp.Code)
			if resp.Code != http.StatusOK {
				return
			}

			var res Result[int64]
			err = json.NewDecoder(resp.Body).Decode(&res)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRes.Code, res.Code)
			if tc.wantRes.Data > 0 {
				assert.True(t, res.Data > 0)
			}

			tc.after(t)
		})
	}
}
