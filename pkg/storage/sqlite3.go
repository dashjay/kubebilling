package storage

import (
	"github.com/dashjay/kubebilling/pkg/records"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"time"
)

type Sqlite struct {
	*gorm.DB
}

func (s *Sqlite) AddPodBaseRecord(pbr *records.PodBaseRecord) error {
	var existsPbr records.PodBaseRecord
	res := s.DB.First(&existsPbr, "uid = ?", pbr.UID)
	if err := res.Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return s.DB.Create(&pbr).Error
		}
	}
	return nil
}

func (s *Sqlite) AddPodUsageRecord(pur *records.PodUsageRecord) error {
	return s.DB.Create(&pur).Error
}

var _ Interface = (*Sqlite)(nil)

func NewSqlite(path string) (Interface, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	err = db.AutoMigrate(&records.PodUsageRecord{})
	if err != nil {
		return nil, err
	}
	sql := &Sqlite{DB: db}
	go sql.vacuumLoop(time.Hour)
	return sql, nil
}

func (s *Sqlite) vacuum() {
	s.DB.Exec("VACUUM")
}

func (s *Sqlite) vacuumLoop(duration time.Duration) {
	ticker := time.NewTicker(duration)
	for range ticker.C {
		s.vacuum()
	}
}
