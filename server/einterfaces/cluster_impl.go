package einterfaces

import (
    "encoding/json"
    "sync"
    "time"

    "github.com/mattermost/mattermost/server/public/model"
    "github.com/mattermost/mattermost/server/public/shared/mlog"
    "github.com/gomodule/redigo/redis"
)

type OpenSourceCluster struct {
    nodeID            string
    redisPool         *redis.Pool
    handlers          map[string][]ClusterMessageHandler
    handlersLock      sync.RWMutex
    leaderListeners   map[string]func()
    leaderListenersMu sync.RWMutex
    logger            *mlog.Logger
}

func NewOpenSourceCluster(redisAddress, redisPassword string) ClusterInterface {
    nodeID := model.NewId()
    
    redisPool := &redis.Pool{
        MaxIdle:     3,
        IdleTimeout: 240 * time.Second,
        Dial: func() (redis.Conn, error) {
            return redis.Dial("tcp", redisAddress,
                redis.DialPassword(redisPassword))
        },
    }
    
    cluster := &OpenSourceCluster{
        nodeID:          nodeID,
        redisPool:       redisPool,
        handlers:        make(map[string][]ClusterMessageHandler),
        leaderListeners: make(map[string]func()),
        logger:          mlog.CreateConsoleLogger(true, mlog.LvlDebug),
    }
    
    // Start listening for cluster messages
    go cluster.listenForClusterMessages()
    
    return cluster
}

func (c *OpenSourceCluster) StartInterNodeCommunication() {
    c.logger.Info("Starting internode communication for cluster", mlog.String("nodeID", c.nodeID))
}

func (c *OpenSourceCluster) StopInterNodeCommunication() {
    c.logger.Info("Stopping internode communication for cluster", mlog.String("nodeID", c.nodeID))
    c.redisPool.Close()
}

func (c *OpenSourceCluster) RegisterClusterMessageHandler(event string, handler ClusterMessageHandler) {
    c.handlersLock.Lock()
    defer c.handlersLock.Unlock()
    
    c.handlers[event] = append(c.handlers[event], handler)
    c.logger.Debug("Registered cluster message handler", mlog.String("event", event))
}

func (c *OpenSourceCluster) SendClusterMessage(message *model.ClusterMessage) {
    message.SentAt = model.GetMillis()
    
    // Set the sender node ID
    if message.Props == nil {
        message.Props = make(map[string]string)
    }
    message.Props["sender_node_id"] = c.nodeID
    
    jsonData, err := json.Marshal(message)
    if err != nil {
        c.logger.Error("Failed to encode cluster message", mlog.Err(err))
        return
    }
    
    conn := c.redisPool.Get()
    defer conn.Close()
    
    _, err = conn.Do("PUBLISH", "cluster:messages", jsonData)
    if err != nil {
        c.logger.Error("Failed to publish cluster message", mlog.Err(err))
    }
}

func (c *OpenSourceCluster) listenForClusterMessages() {
    conn := c.redisPool.Get()
    defer conn.Close()
    
    psc := redis.PubSubConn{Conn: conn}
    psc.Subscribe("cluster:messages")
    
    for {
        switch n := psc.Receive().(type) {
        case redis.Message:
            var msg model.ClusterMessage
            if err := json.Unmarshal(n.Data, &msg); err != nil {
                c.logger.Error("Failed to decode cluster message", mlog.Err(err))
                continue
            }
            
            // Don't process messages from this node
            if msg.Props != nil && msg.Props["sender_node_id"] == c.nodeID {
                continue
            }
            
            c.handlersLock.RLock()
            handlers, ok := c.handlers[msg.Event]
            c.handlersLock.RUnlock()
            
            if !ok {
                continue
            }
            
            for _, handler := range handlers {
                go handler(&msg)
            }
            
        case error:
            c.logger.Error("Error in Redis subscription", mlog.Err(n))
            
            // Reconnect
            psc.Close()
            conn = c.redisPool.Get()
            psc = redis.PubSubConn{Conn: conn}
            psc.Subscribe("cluster:messages")
        }
    }
}

func (c *OpenSourceCluster) AddClusterLeaderChangedListener(listener func()) string {
    id := model.NewId()
    
    c.leaderListenersMu.Lock()
    defer c.leaderListenersMu.Unlock()
    
    c.leaderListeners[id] = listener
    
    return id
}

func (c *OpenSourceCluster) RemoveClusterLeaderChangedListener(id string) {
    c.leaderListenersMu.Lock()
    defer c.leaderListenersMu.Unlock()
    
    delete(c.leaderListeners, id)
}

func (c *OpenSourceCluster) InvokeClusterLeaderChangedListeners() {
    c.leaderListenersMu.RLock()
    defer c.leaderListenersMu.RUnlock()
    
    for _, listener := range c.leaderListeners {
        go listener()
    }
}
