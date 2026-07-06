# -*- coding: utf-8 -*-
import argparse
import json
import os
from datetime import datetime
from typing import List, Dict

import requests

try:
    import docker
except ImportError:
    docker = None


# feat: 配置管理抽取出来类中专门管理
class ConfigManager:
    @staticmethod
    def load_config(file_path: str) -> Dict:
        """加载配置文件"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                config = json.load(f)
                required_keys = ['name', 'url', 'members']
                for wh in config.get('webhooks', []):
                    if not all(k in wh for k in required_keys):
                        raise ValueError(f"Webhook配置缺少必要字段: {wh}")
                return config
        except Exception as e:
            print(f"file_path:{file_path},配置加载失败: {str(e)}", flush=True)
            exit(1)


class NotificationSender:
    def __init__(self, config: Dict):
        self.webhooks = config.get('webhooks', [])
        self.role = config.get('role', 'tester')
        self.env_name = config.get('env_name', 'DEMO环境')

    def generate_payload_template(
            self,
            current_path: str,
            modules: str,
            username: str,
            show_all_containers: bool
    ) -> Dict:
        """
        :type show_all_containers: bool 是否展示所有容器
        :type username: str 用户名
        :type modules: str 模块列表
        :type current_path: str 路径
        :rtype: 入参payload模板
        """
        # 处理模块列表参数
        module_list = [m.strip() for m in modules.split(',') if m.strip()]
        if not module_list:
            print("说明：--modules的有效参数为空,无需推送!", flush=True)
            exit(1)

        # 获取格式化时间
        update_time = datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S%z")

        facts = [
            {"title": "🌍更新环境", "value": self.env_name},
            {"title": "📦更新模块", "value": ", ".join(module_list)},
            {"title": "🕒更新时间", "value": f"{update_time} (Server Time)"},
        ]

        # 构造推送数据
        return {
            "type": "message",
            "attachments": [
                {
                    "contentType": "application/vnd.microsoft.card.adaptive",
                    "contentUrl": "",
                    "content": {
                        "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
                        "type": "AdaptiveCard",
                        "version": "1.5",
                        "body": [
                            {
                                "type": "TextBlock",
                                "text": "反馈：🚨SPUG更新完成通知",
                                "wrap": True,
                                "weight": "bolder",
                                "size": "medium",
                                "color": "Attention",
                                "style": "heading"
                            },
                            {
                                "type": "TextBlock",
                                "text": "<at>所有人</at>, 请知悉！",
                                "wrap": True
                            },
                            {
                                "type": "FactSet",
                                "facts": facts
                            }
                        ],
                        "msteams": {
                            "entities": []
                        }
                    }
                }
            ]
        }

    def send_notifications(self, template: Dict):
        """发送所有通知"""
        for webhook in self.webhooks:
            print(
                f"\n正在处理 webhook: {webhook['name']}, description: {webhook['description']}, url: {webhook['url']}",
                flush=True
            )

            payload = json.loads(json.dumps(template))

            # 发送POST请求
            try:
                print(f"推送Teams群聊卡片请求参数: {payload}", flush=True)
                requests.post(
                    url=webhook['url'],
                    json=payload,
                    headers={"Content-Type": "application/json"},
                    timeout=10
                )

            except Exception as e:
                print(f"⚠️Microsoft Teams通知异常:{str(e)}", flush=True)

    @staticmethod
    def generate_mentions(members: List[Dict]) -> tuple:
        """生成@提及的消息组件
        :param members:成员
        :return: teams的mentions
        """
        mention_text = []
        entities = []
        for member in members:
            tag = f"<at>{member['name']}</at>"
            mention_text.append(tag)
            entities.append({
                "type": "mention",
                "text": tag,
                "mentioned": {
                    "id": member["id"],
                    "name": member["name"]
                }
            })
        return " ".join(mention_text), entities


class EnvInfoGetter:
    @staticmethod
    def get_username(args, current_path: str) -> str:
        """获取用户名（优先从参数获取，参数没有取工作空间的）"""
        # 优先使用命令行参数
        if args.username:
            return args.username

        # 从工作路径解析
        try:
            path_parts = current_path.split(os.sep)
            prosh_index = path_parts.index("prosh")
            return path_parts[prosh_index + 1]
        except ValueError:
            raise RuntimeError("工作路径中未找到/prosh/目录结构")
        except IndexError:
            raise RuntimeError("工作路径中/prosh/后缺少用户名目录")
        except Exception as e:
            print(f"错误：工作路径必须符合/prosh/用户名格式,<UNK>:{str(e)}", flush=True)
            exit(1)


def main():
    print("==> Begin Frank Update Tools(version: 3.0) Notify Microsoft Teams Python Script!", flush=True)
    # 解析命令行参数
    parser = argparse.ArgumentParser(description='发送Microsoft Teams程序更新通知')
    parser.add_argument('--modules', required=True,
                        help='更新模块名称，多个用逗号分隔（例：app,pm）')
    parser.add_argument('--username', required=False,
                        help='更新人员名称,默认读取/prosh/xxx工作空间中的xxx姓名')
    parser.add_argument('--config', default='settings.json',
                        help='配置文件路径，默认为settings.json')
    parser.add_argument("--show-all-containers", action='store_true',
                        help='显示所有Docker容器状态（与--docker-modules互斥）')
    args = parser.parse_args()

    # 获取基础环境参数,获取pwd和username
    current_path = os.getcwd()
    username = EnvInfoGetter.get_username(args, current_path)

    # 获取settings配置文件
    config = ConfigManager.load_config(args.config)
    # 根据配置进行通知
    sender = NotificationSender(config)
    # 获取请求模板
    payload_template = sender.generate_payload_template(
        current_path,
        args.modules,
        username,
        args.show_all_containers
    )
    # 进行请求
    sender.send_notifications(payload_template)
    print("<== Finish Frank Update Tools(version: 3.0) Notify Microsoft Teams Python Script!", flush=True)


if __name__ == "__main__":
    main()
