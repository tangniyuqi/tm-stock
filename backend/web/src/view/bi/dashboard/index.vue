<template>
  <div class="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-12 py-2 gap-4 md:gap-2 gva-container2">
    <gva-card custom-class="col-span-1 lg:col-span-3">
      <gva-chart :type="1" title="今日数据量" />
    </gva-card>
 
    <gva-card custom-class="col-span-1 lg:col-span-3">
      <gva-chart :type="2" title="本周数据量" />
    </gva-card>

    <gva-card custom-class="col-span-1 lg:col-span-3">
      <gva-chart :type="3" title="本月数据量" />
    </gva-card>

    <gva-card custom-class="col-span-1 lg:col-span-3">
      <gva-chart :type="3" title="全部数据量" />
    </gva-card>

    <gva-card title="消息数据" custom-class="col-span-1 md:col-span-2 md:row-start-2 lg:col-span-12 col-start-1 row-span-2 ">
      <gva-chart :type="4" />
    </gva-card>

    <gva-card title="用户数据" custom-class="lg:col-span-12">
      <gva-chart :type="4" />
    </gva-card>
  </div>
</template>

<script setup>
  import {
    GvaChart,
    GvaCard,
  } from './components'
  defineOptions({
    name: 'Dashboard'
  })
</script>

<style lang="scss" scoped></style>
