package article

import (
	"context"
	"errors"
	"fmt"
	"github.com/johnwongx/webook/backend/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type GORMArticleDAO struct {
	db *gorm.DB
	l  logger.Logger
}

func NewGORMArticleDAO(db *gorm.DB, l logger.Logger) ArticleDAO {
	return &GORMArticleDAO{
		db: db,
		l:  l,
	}
}

func (g *GORMArticleDAO) Insert(ctx context.Context, art Article) (int64, error) {
	now := time.Now().UnixMilli()
	art.Ctime = now
	art.Utime = now
	err := g.db.WithContext(ctx).Create(&art).Error
	return art.Id, err
}

func (g *GORMArticleDAO) UpdateById(ctx context.Context, art Article) error {
	now := time.Now().UnixMilli()
	art.Utime = now
	res := g.db.Model(&Article{}).WithContext(ctx).
		Where("id=? AND author_id=?", art.Id, art.AuthorId).
		Updates(map[string]any{
			"title":   art.Title,
			"content": art.Content,
			"utime":   art.Utime,
			"status":  art.Status,
		})
	err := res.Error
	if err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return errors.New("更新数据失败")
	}
	return nil
}

func (g *GORMArticleDAO) Sync(ctx context.Context, art Article) (int64, error) {
	var (
		id  = art.Id
		err error
	)

	// 使用事物保证两张表同时成功或失败
	err = g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新制作库，插入或删除
		if id > 0 {
			err = g.UpdateById(ctx, art)
		} else {
			id, err = g.Insert(ctx, art)
		}
		if err != nil {
			return err
		}
		// 更新数据到线上库
		art.Id = id
		return g.Upsert(ctx, PublishArticle(art))
	})
	return id, err
}

func (g *GORMArticleDAO) Upsert(ctx context.Context, art PublishArticle) error {
	now := time.Now().UnixMilli()
	art.Ctime = now
	art.Utime = now
	return g.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]interface{}{
				"title":   art.Title,
				"content": art.Content,
				"utime":   art.Utime,
				"status":  art.Status,
			}),
		}).Create(&art).Error
}

func (g *GORMArticleDAO) SyncStatus(ctx context.Context, id, usrId int64, status uint8) error {
	now := time.Now().UnixMilli()
	err := g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&Article{}).
			Where("id = ? AND author_id = ?", id, usrId).
			Updates(map[string]any{
				"status": status,
				"utime":  now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("可能有人在攻击系统，误操作非自己的文章, id:%d, authorId:%d", id, usrId)
		}

		return tx.Model(&PublishArticle{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"status": status,
				"utime":  now,
			}).Error
	})
	return err
}

func (g *GORMArticleDAO) GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]Article, error) {
	var arts []Article
	err := g.db.WithContext(ctx).Model(&Article{}).
		Where("author_id = ?", uid).
		Offset(offset).
		Limit(limit).
		Order("utime DESC").
		Find(arts).Error
	return arts, err
}

func (g *GORMArticleDAO) FindById(ctx context.Context, id, uid int64) (Article, error) {
	var art Article
	err := g.db.WithContext(ctx).Model(&Article{}).
		Where("id = ? AND author_id = ?", id, uid).
		First(&art).Error
	if err != nil {
		g.l.Error(fmt.Sprintf("可能有人在攻击系统，误操作非自己的文章, id:%d, authorId:%d", id, uid), logger.Error(err))
		return Article{}, err
	}
	return art, nil
}

func (g *GORMArticleDAO) FindPubById(ctx context.Context, id int64) (PublishArticle, error) {
	var art PublishArticle
	err := g.db.WithContext(ctx).Model(&PublishArticle{}).
		Where("id = ?", id).
		First(&art).Error
	if err != nil {
		return PublishArticle{}, err
	}
	return art, nil
}
