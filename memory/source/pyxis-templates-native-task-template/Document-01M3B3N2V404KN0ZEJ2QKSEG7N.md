{
  "schema_version": "native-task-input/v1",
  "title": "Describe the task",
  "goal": "Describe the behavior to deliver",
  "scope": "Describe the included change",
  "acceptance": ["Describe an observable passing result"],
  "native_routing": {
    "schema_version": "task-native-routing/v1",
    "implementation": {"harness": "codex", "model": "gpt-5.6-sol", "effort": "high"},
    "review": {"harness": "claude", "model": "claude-opus-5", "effort": "max"},
    "rationale": "Explain why these two native choices suit this task"
  }
}
