<p align="center">
    <h3 align="center">SmartPing | 开源、高效、便捷的网络质量监控神器</h3>
    <p align="center">
       一个综合性网络质量(PING)检测工具，支持正/反向PING绘图、互PING拓扑绘图与报警、全国PING延迟地图与在线检测工具等功能
        <br>
        <br>
        <a href="https://github.com/Antman2023/smartping-next/releases">
            <img src="https://img.shields.io/github/release/Antman2023/smartping-next.svg" >
        </a>
        <a href="https://github.com/Antman2023/smartping-next/blob/master/LICENSE">
            <img src="https://img.shields.io/hexpm/l/plug.svg" >
        </a>
    </p>
</p>

## 功能

- 正向PING，反向Ping绘图
- 互PING间机器的状态拓扑，自定义延迟、丢包阈值报警（声��报警与邮件报警），报警时MTR检测
- 全国PING延迟地图（各省份可分电信、联通、移动三条线路）
- 检测工具，支持使用SmartPing各节点进行网络相关检测
- 支持深色/浅色主题切换

## 技术栈

- **后端**: Go 1.24 + SQLite3
- **前端**: Vue 3 + TypeScript + Vite + Element Plus + ECharts

## 快速开始

### Linux/macOS
```bash
./control build    # 编译
./control run      # 前台运行
./control start    # 后台启动
./control stop     # 停止
./control restart  # 重启
./control status   # 查看状态
./control pack     # 打包发布
```

### Windows
```cmd
control.cmd        # 交互式菜单（build/run/install/start/stop/restart）
```

### 手动构建
```bash
# 后端
go get -v ./...
go build -o bin/smartping src/smartping.go

# 前端
cd web
npm install
npm run build
```

**默认端口**: 18899 | **默认密码**: smartping

## 设计思路

本系统的定位为轻量级工具，即使组多点成互Ping网络可以遵守无中心化原则，所有的数据均存储自身节点中，每个节点提供出方向的数据，从任意节点查询数据均会通过Ajax请求关联节点的API接口获取并组装全部数据。

## 目录结构

```
├── src/                    # Go 后端源码
│   ├── smartping.go        # 程序入口
│   ├── g/                  # 全局配置和数据结构
│   ├── http/               # HTTP 服务层
│   ├── funcs/              # 核心业务逻辑
│   └── nettools/           # 底层网络工具
├── web/                    # Vue 3 前端
│   ├── src/
│   │   ├── views/          # 页面组件
│   │   ├── components/     # 通用组件
│   │   ├── api/            # API 接口
│   │   └── assets/         # 静态资源
│   └── package.json
├── conf/                   # 配置文件
└── db/                     # SQLite 数据库
```

## API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/config.json` | GET | 获取配置 |
| `/api/ping.json` | GET | 获取 PING 数据 |
| `/api/topology.json` | GET | 获取拓扑状态 |
| `/api/alert.json` | GET | 获取报警日志 |
| `/api/mapping.json` | GET | 获取地图数据 |
| `/api/tools.json` | GET | 在线检测工具 |
| `/api/saveconfig.json` | POST | 保存配置 |
| `/api/proxy.json` | GET | 代理访问远程节点 |

## 项目贡献

欢迎参与项目贡献！比如提交PR修复一个bug，或者新建 [Issue](https://github.com/Antman2023/smartping-next/issues/) 讨论新特性或者变更。

## 致谢

本项目基于 [smartping/smartping](https://github.com/smartping/smartping) 开发，在原有功能基础上进行了以下改进：

- 使用 Vue 3 + TypeScript + Element Plus 重构前端
- 新增深色/浅色主题切换支持
- 优化图表渲染性能
- 改进响应式布局适配
