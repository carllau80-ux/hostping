package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoProvider MongoDB（官方 mongo-driver v2，BSON 协议纯 Go）。
type mongoProvider struct{}

func (mongoProvider) client(c *model.Conn, readTimeout time.Duration) (*mongo.Client, error) {
	authDB := c.Database
	if authDB == "" {
		authDB = "admin"
	}
	u := &url.URL{Scheme: "mongodb", Host: netAddr(c)}
	if c.User != "" {
		u.User = url.UserPassword(c.User, c.Password)
		u.Path = "/" + authDB // authSource
	}
	q := url.Values{}
	q.Set("connectTimeoutMS", strconv.Itoa(timeoutSecs(c)*1000))
	q.Set("serverSelectionTimeoutMS", strconv.Itoa(timeoutSecs(c)*1000))
	q.Set("socketTimeoutMS", strconv.Itoa(int(readTimeout.Seconds()*1000)))
	q.Set("appName", "dbping")
	for k, v := range parseParams(c.Params) {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	opts := options.Client().ApplyURI(u.String())
	return mongo.Connect(opts)
}

// runCmd 执行管理命令并解码为 map。
func runCmd(ctx context.Context, db *mongo.Database, cmd any) (bson.M, error) {
	var doc bson.M
	if err := db.RunCommand(ctx, cmd).Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (mongoProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	res := &model.PingResult{ConnID: c.ID, Stage: "tcp", CheckedAt: time.Now().UnixMilli()}
	start := time.Now()
	tcpMs, err := DialTCP(c)
	if err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.TCPMs = tcpMs

	ctx, cancel := DBCtx(c)
	defer cancel()
	client, err := mongoProvider{}.client(c, timeoutDur(c)*2)
	if err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	defer client.Disconnect(context.Background())

	res.Stage = "auth"
	if err := client.Ping(ctx, nil); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.Stage = "sql"
	s := time.Now()
	if _, err := runCmd(ctx, client.Database("admin"), bson.M{"ping": 1}); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.SQLMs = time.Since(s).Milliseconds()
	if res.SQLMs <= 0 {
		res.SQLMs = 1
	}
	res.Stage = "ok"
	res.OK = true
	res.TotalMs = time.Since(start).Milliseconds()
	return res, nil
}

func (mongoProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	ctx, cancel := DBCtx(c)
	defer cancel()
	client, err := mongoProvider{}.client(c, timeoutDur(c)*2)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.Background())
	admin := client.Database("admin")
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// buildInfo → 版本
	if doc, err := runCmd(ctx, admin, bson.M{"buildInfo": 1}); err == nil {
		o.Version = fmt.Sprint(doc["version"])
		o.VersionFull = "MongoDB " + o.Version
		if modules, ok := doc["modules"].(bson.A); ok && len(modules) > 0 {
			o.VersionFull += fmt.Sprintf("（%v）", modules)
		}
	}

	// serverStatus → 运行状态（需要 clusterMonitor 权限）
	var st bson.M
	if doc, err := runCmd(ctx, admin, bson.M{"serverStatus": 1}); err == nil {
		st = doc
	} else {
		o.Warnings = append(o.Warnings, "serverStatus 需要更高权限（clusterMonitor），部分信息缺失: "+shortErr(err))
	}
	if st != nil {
		o.Hostname = fmt.Sprint(st["host"])
		o.Instance = fmt.Sprint(st["process"])
		if up, ok := st["uptime"].(int32); ok {
			o.Uptime = HumanDuration(int64(up))
		}
		if conns, ok := st["connections"].(bson.M); ok {
			o.ConnCur = fmt.Sprint(conns["current"])
			if avail, err := strconv.ParseInt(fmt.Sprint(conns["available"]), 10, 64); err == nil {
				cur, _ := strconv.ParseInt(fmt.Sprint(conns["current"]), 10, 64)
				o.ConnMax = strconv.FormatInt(cur+avail, 10)
			}
		}
		if se, ok := st["storageEngine"].(bson.M); ok {
			o.ArchiveMode = fmt.Sprint(se["name"]) + " 存储引擎"
		}
	}

	// hostInfo → 主机与操作系统
	if doc, err := runCmd(ctx, admin, bson.M{"hostInfo": 1}); err == nil {
		if sys, ok := doc["system"].(bson.M); ok {
			if o.Hostname == "" {
				o.Hostname = fmt.Sprint(sys["hostname"])
			}
		}
		if osv, ok := doc["os"].(bson.M); ok {
			o.OS = strings.TrimSpace(fmt.Sprint(osv["name"]) + " " + fmt.Sprint(osv["version"]))
		}
		if arch, ok := doc["system"].(bson.M); ok {
			if a := fmt.Sprint(arch["cpuArch"]); a != "<nil>" {
				o.OS = strings.TrimSpace(o.OS + " " + a)
			}
		}
	}

	// isMaster / hello → 副本集角色
	if doc, err := runCmd(ctx, admin, bson.M{"hello": 1}); err == nil {
		if setName, ok := doc["setName"].(string); ok && setName != "" {
			o.Cluster = "副本集 " + setName
			isMaster, _ := doc["ismaster"].(bool)
			switch {
			case isMaster:
				o.Role = "主节点（Primary）"
			case doc["secondary"] == true:
				o.Role = "从节点（Secondary）"
			default:
				o.Role = "副本集成员"
			}
		} else {
			o.Cluster = "单机"
			o.Role = "单机"
		}
	} else if doc, err := runCmd(ctx, admin, bson.M{"isMaster": 1}); err == nil {
		if setName, ok := doc["setName"].(string); ok && setName != "" {
			o.Cluster = "副本集 " + setName
		}
	}

	// 库列表与总大小
	if res, err := client.ListDatabases(ctx, bson.M{}); err == nil {
		total := int64(0)
		n := 0
		for _, dbi := range res.Databases {
			total += dbi.SizeOnDisk
			n++
		}
		o.DBSize = HumanBytes(total)
		o.Extra = append(o.Extra, model.KV{Key: "数据库数量", Value: strconv.Itoa(n)})
	}

	o.Charset = "UTF-8"
	o.Timezone = "-"
	o.ReadOnly = "-"
	if o.TableCount == "" {
		o.TableCount = "-"
	}
	if o.Role == "" {
		o.Role = "-"
	}
	return o, nil
}

func (mongoProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	ctx, cancel := DBCtx(c)
	defer cancel()
	client, err := mongoProvider{}.client(c, timeoutDur(c)*2)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.Background())
	d := &model.Detail{}

	// 库列表
	lr, err := client.ListDatabases(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	collectTop := 0
	for _, dbi := range lr.Databases {
		note := ""
		if dbi.Empty {
			note = "（空库）"
		}
		if dbi.Name == "admin" || dbi.Name == "config" || dbi.Name == "local" {
			note += "（系统库）"
		}
		entry := model.DbInfo{Name: dbi.Name, Size: HumanBytes(dbi.SizeOnDisk), Note: note}

		// 前 3 个业务库枚举集合并统计数量
		if collectTop < 3 && dbi.Name != "admin" && dbi.Name != "config" && dbi.Name != "local" && !dbi.Empty {
			collectTop++
			if colls, err := client.Database(dbi.Name).ListCollectionNames(ctx, nil); err == nil {
				entry.Tables = fmt.Sprintf("%d", len(colls))
				for i, cn := range colls {
					if i >= 50 {
						break
					}
					d.Tables = append(d.Tables, model.TableInfo{Schema: dbi.Name, Name: cn})
				}
			}
		}
		d.Dbs = append(d.Dbs, entry)
	}

	// 副本集成员
	if doc, err := runCmd(ctx, client.Database("admin"), bson.M{"replSetGetStatus": 1}); err == nil {
		if members, ok := doc["members"].(bson.A); ok {
			for _, mm := range members {
				if m, ok := mm.(bson.M); ok {
					d.Services = append(d.Services, model.SvcInfo{
						Name:   fmt.Sprint(m["name"]),
						Role:   "副本集成员",
						Status: fmt.Sprint(m["stateStr"]),
						Host:   fmt.Sprint(m["name"]),
						Detail: "health=" + fmt.Sprint(m["health"]),
					})
				}
			}
		}
	}
	return d, nil
}

func (mongoProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	ctx, cancel := DBCtx(c)
	defer cancel()
	client, err := mongoProvider{}.client(c, timeoutDur(c)*2)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.Background())
	admin := client.Database("admin")

	root := newTNode("root", c.Name, "MongoDB @ "+c.Host+":"+itoa(c.Port), "ok")

	// 副本集成员分支
	if doc, err := runCmd(ctx, admin, bson.M{"replSetGetStatus": 1}); err == nil {
		if setName, ok := doc["set"].(string); ok {
			br := addNode(root, newTNode("rs", "副本集 "+setName, "", "ok"))
			if members, ok := doc["members"].(bson.A); ok {
				for i, mm := range members {
					if m, ok := mm.(bson.M); ok {
						st := "ok"
						if fmt.Sprint(m["health"]) != "1" || fmt.Sprint(m["stateStr"]) != "PRIMARY" && fmt.Sprint(m["stateStr"]) != "SECONDARY" {
							st = "warn"
						}
						addNode(br, newTNode(fmt.Sprintf("m-%d", i), fmt.Sprint(m["name"]),
							fmt.Sprint(m["stateStr"]), st))
					}
				}
			}
		}
	}

	// 库 → 集合
	if lr, err := client.ListDatabases(ctx, bson.M{}); err == nil {
		br := addNode(root, newTNode("dbs", "数据库", fmt.Sprintf("%d 个", len(lr.Databases)), "na"))
		for i, dbi := range lr.Databases {
			n := addNode(br, newTNode(fmt.Sprintf("db-%d", i), dbi.Name, HumanBytes(dbi.SizeOnDisk), "ok"))
			if dbi.Empty {
				continue
			}
			if colls, err := client.Database(dbi.Name).ListCollectionNames(ctx, nil); err == nil {
				for j, cn := range colls {
					if j >= 10 {
						addNode(n, newTNode(fmt.Sprintf("db-%d-more", i), fmt.Sprintf("…共 %d 个集合", len(colls)), "", "na"))
						break
					}
					addNode(n, newTNode(fmt.Sprintf("db-%d-c-%d", i, j), cn, "", "na"))
				}
			}
		}
	}
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
