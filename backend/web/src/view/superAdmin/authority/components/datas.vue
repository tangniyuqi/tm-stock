<template>
  <el-alert
    title="旧版角色关系设置已停用"
    description="请在角色管理页使用现有的数据权限和部门集设置。本页不会提交或更改任何角色权限。"
    type="info"
    :closable="false"
    show-icon
  />
</template>

<script setup>
defineOptions({ name: 'Datas' })

// 保留旧容器的切页契约，不恢复已删除的角色关系接口或发出写请求。
defineExpose({ needConfirm: false, enterAndNext: () => {} })
</script>
