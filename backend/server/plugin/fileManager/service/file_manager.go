package service

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/fileManager/model"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/fileManager/model/request"
	"github.com/gin-gonic/gin"
)

type FMS struct{}

// UploadFile 上传文件
func (s *FMS) UploadFile(header *multipart.FileHeader, params request.FileUpload, userId uint) (record model.PluginFileRecord, err error) {
	// 1. 获取本地存储根路径
	storePath := global.GVA_CONFIG.Local.StorePath
	if storePath == "" {
		storePath = "uploads/file"
	}

	// 2. 构建子目录路径，如果不存在则创建
	subPath := filepath.Join(storePath, params.Path)
	if err = os.MkdirAll(subPath, os.ModePerm); err != nil {
		return record, errors.New("创建目录失败: " + err.Error())
	}

	// 3. 生成新的文件名
	ext := filepath.Ext(header.Filename)
	baseName := strings.TrimSuffix(header.Filename, ext)
	filename := fmt.Sprintf("%s_%d%s", baseName, time.Now().Unix(), ext)
	fullPath := filepath.Join(subPath, filename)

	// 4. 保存文件到磁盘
	src, err := header.Open()
	if err != nil {
		return record, errors.New("打开文件失败: " + err.Error())
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return record, errors.New("创建文件失败: " + err.Error())
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return record, errors.New("保存文件失败: " + err.Error())
	}

	// 5. 获取文件MIME类型
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// 6. 构建访问URL
	relativePath := filepath.Join(params.Path, filename)
	fileUrl := "/" + strings.ReplaceAll(relativePath, "\\", "/")

	// 7. 将文件元数据存入数据库
	record = model.PluginFileRecord{
		Name:       header.Filename,
		Key:        relativePath,
		Url:        fileUrl,
		Path:       params.Path,
		Tag:        strings.TrimPrefix(ext, "."),
		Size:       header.Size,
		MimeType:   mimeType,
		CategoryId: params.CategoryId,
		UploaderId: userId,
		Remark:     params.Remark,
	}

	err = global.GVA_DB.Create(&record).Error
	return record, err
}

// GetFileList 获取文件列表
func (s *FMS) GetFileList(params request.FileSearch) (list []model.PluginFileRecord, total int64, err error) {
	db := global.GVA_DB.Model(&model.PluginFileRecord{})

	// 构建查询条件
	if params.Name != "" {
		db = db.Where("name LIKE ?", "%"+params.Name+"%")
	}
	if params.Path != "" {
		db = db.Where("path = ?", params.Path)
	}
	if params.CategoryId > 0 {
		db = db.Where("category_id = ?", params.CategoryId)
	}
	if params.Tag != "" {
		db = db.Where("tag = ?", params.Tag)
	}
	if params.StartTime != "" && params.EndTime != "" {
		db = db.Where("created_at BETWEEN ? AND ?", params.StartTime, params.EndTime)
	}

	// 分页
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	err = db.Limit(params.PageSize).Offset((params.Page - 1) * params.PageSize).Order("id desc").Find(&list).Error
	return
}

// GetFileById 根据ID获取文件
func (s *FMS) GetFileById(id uint) (record model.PluginFileRecord, err error) {
	err = global.GVA_DB.First(&record, id).Error
	return
}

// UpdateFile 更新文件信息
func (s *FMS) UpdateFile(params request.FileUpdate) error {
	return global.GVA_DB.Model(&model.PluginFileRecord{}).
		Where("id = ?", params.ID).
		Updates(map[string]interface{}{
			"name":   params.Name,
			"remark": params.Remark,
		}).Error
}

// DeleteFiles 批量删除文件
func (s *FMS) DeleteFiles(ids []uint) error {
	var records []model.PluginFileRecord
	if err := global.GVA_DB.Find(&records, ids).Error; err != nil {
		return err
	}

	// 删除物理文件
	storePath := global.GVA_CONFIG.Local.StorePath
	if storePath == "" {
		storePath = "uploads/file"
	}

	for _, record := range records {
		filePath := filepath.Join(storePath, record.Key)
		_ = os.Remove(filePath) // 忽略删除失败的错误
	}

	// 删除数据库记录
	return global.GVA_DB.Delete(&model.PluginFileRecord{}, ids).Error
}

// DownloadFile 下载文件
func (s *FMS) DownloadFile(c *gin.Context, id uint, disposition string) error {
	// 获取文件记录
	record, err := s.GetFileById(id)
	if err != nil {
		return errors.New("文件不存在")
	}

	// 构建文件完整路径
	storePath := global.GVA_CONFIG.Local.StorePath
	if storePath == "" {
		storePath = "uploads/file"
	}
	filePath := filepath.Join(storePath, record.Key)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return errors.New("文件已被删除")
	}

	// 设置响应头
	if disposition != "inline" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, url.QueryEscape(record.Name)))
	c.Header("Content-Type", record.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", record.Size))

	// 发送文件
	c.File(filePath)
	return nil
}

// CreateCategory 新建分类
func (s *FMS) CreateCategory(params request.CategoryCreate, createdBy uint) error {
	category := &model.PluginFileCategory{
		Name:      params.Name,
		ParentId:  params.ParentId,
		CreatedBy: createdBy,
	}
	// TODO: 计算path和level
	return global.GVA_DB.Create(category).Error
}

// UpdateCategory 更新分类
func (s *FMS) UpdateCategory(params request.CategoryUpdate) error {
	// 检查目标父分类是否是其自身或其子分类
	var category model.PluginFileCategory
	if err := global.GVA_DB.First(&category, params.ID).Error; err != nil {
		return errors.New("分类不存在")
	}

	if params.ParentId > 0 {
		var current model.PluginFileCategory
		if err := global.GVA_DB.First(&current, params.ParentId).Error; err != nil {
			return errors.New("父分类不存在")
		}
		// 检查是否将分类移动到其子分类下
		var children []model.PluginFileCategory
		if err := global.GVA_DB.Where("path LIKE ?", category.Path+"%").Find(&children).Error; err == nil {
			for _, child := range children {
				if child.ID == params.ParentId {
					return errors.New("不能将分类移动到其子分类下")
				}
			}
		}
	}

	return global.GVA_DB.Model(&model.PluginFileCategory{}).
		Where("id = ?", params.ID).
		Updates(map[string]interface{}{
			"name":      params.Name,
			"parent_id": params.ParentId,
		}).Error
}

// DeleteCategory 删除分类
func (s *FMS) DeleteCategory(id uint) error {
	// 检查是否有子分类
	var childCount int64
	global.GVA_DB.Model(&model.PluginFileCategory{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return errors.New("该分类下有子分类，无法删除")
	}

	// 检查分类下是否有文件
	var fileCount int64
	global.GVA_DB.Model(&model.PluginFileRecord{}).Where("category_id = ?", id).Count(&fileCount)
	if fileCount > 0 {
		return errors.New("该分类下有文件，无法删除")
	}

	return global.GVA_DB.Delete(&model.PluginFileCategory{}, id).Error
}

// GetCategoryTree 获取分类树
func (s *FMS) GetCategoryTree() (tree []model.PluginFileCategory, err error) {
	var allCategories []model.PluginFileCategory
	err = global.GVA_DB.Order("sort asc, id asc").Find(&allCategories).Error
	if err != nil {
		return nil, err
	}

	// 使用递归函数构建树
	tree = buildTree(allCategories, 0)
	return tree, nil
}

// buildTree 递归构建树形结构
func buildTree(categories []model.PluginFileCategory, parentId uint) []model.PluginFileCategory {
	var tree []model.PluginFileCategory
	for _, category := range categories {
		if category.ParentId == parentId {
			// 找到当前父节点的所有子节点
			children := buildTree(categories, category.ID)
			if children != nil {
				category.Children = children
			}
			tree = append(tree, category)
		}
	}
	return tree
}
 