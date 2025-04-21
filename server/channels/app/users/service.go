// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package users

import (
	"errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

type UserService struct {
	store        store.UserStore
	sessionStore store.SessionStore
	oAuthStore   store.OAuthStore
	metrics      einterfaces.MetricsInterface
	cluster      einterfaces.ClusterInterface
	config       func() *model.Config
	license      func() *model.License
}

// ServiceConfig is used to initialize the UserService.
type ServiceConfig struct {
	// Mandatory fields
	UserStore    store.UserStore
	SessionStore store.SessionStore
	OAuthStore   store.OAuthStore
	ConfigFn     func() *model.Config
	LicenseFn    func() *model.License
	// Optional fields
	Metrics einterfaces.MetricsInterface
	Cluster einterfaces.ClusterInterface
}

func New(c ServiceConfig) (*UserService, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	return &UserService{
		store:        c.UserStore,
		sessionStore: c.SessionStore,
		oAuthStore:   c.OAuthStore,
		config:       c.ConfigFn,
		license:      c.LicenseFn,
		metrics:      c.Metrics,
		cluster:      c.Cluster,
	}, nil
}

func (c *ServiceConfig) validate() error {
	if c.ConfigFn == nil || c.UserStore == nil || c.SessionStore == nil || c.OAuthStore == nil || c.LicenseFn == nil {
		return errors.New("required parameters are not provided")
	}

	return nil
}

func (s *UserService) BroadcastStatusChange(userID, status string) {
    if s.cluster == nil {
        return
    }
    
    statusModel := model.Status{
        UserId: userID,
        Status: status,
    }
    
    data, err := json.Marshal(statusModel)
    if err != nil {
        return
    }
    
    msg := &model.ClusterMessage{
        Event:    model.ClusterEventUpdateStatus,
        SendType: model.ClusterSendReliable,
        Data:     data,
    }
    
    s.cluster.SendClusterMessage(msg)
}

func (s *UserService) SetStatusAway(userID string, manual bool) (*model.Status, *model.AppError) {
    status, err := s.store.SaveOrUpdate(&model.Status{
        UserId:         userID,
        Status:         model.StatusAway,
        Manual:         manual,
        LastActivityAt: model.GetMillis(),
    })
    
    if err != nil {
        return nil, err
    }
    
    // Broadcast status change to cluster
    s.BroadcastStatusChange(userID, model.StatusAway)
    
    return status, nil
}

func (s *UserService) SetStatusOnline(userID string, manual bool) (*model.Status, *model.AppError) {
    status, err := s.store.SaveOrUpdate(&model.Status{
        UserId:         userID,
        Status:         model.StatusOnline,
        Manual:         manual,
        LastActivityAt: model.GetMillis(),
    })
    
    if err != nil {
        return nil, err
    }
    
    // Broadcast status change to cluster
    s.BroadcastStatusChange(userID, model.StatusOnline)
    
    return status, nil
}

func (s *UserService) SetStatusDnd(userID string, manual bool) (*model.Status, *model.AppError) {
    status, err := s.store.SaveOrUpdate(&model.Status{
        UserId:         userID,
        Status:         model.StatusDnd,
        Manual:         manual,
        LastActivityAt: model.GetMillis(),
    })
    
    if err != nil {
        return nil, err
    }
    
    // Broadcast status change to cluster
    s.BroadcastStatusChange(userID, model.StatusDnd)
    
    return status, nil
}


func (s *UserService) SetStatusOffline(userID string, manual bool) (*model.Status, *model.AppError) {
    status, err := s.store.SaveOrUpdate(&model.Status{
        UserId:         userID,
        Status:         model.StatusOffline,
        Manual:         manual,
        LastActivityAt: model.GetMillis(),
    })
    
    if err != nil {
        return nil, err
    }
    
    // Broadcast status change to cluster
    s.BroadcastStatusChange(userID, model.StatusOffline)
    
    return status, nil
}