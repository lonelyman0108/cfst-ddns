<script setup lang="ts">
import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

// 正文中的行内链接/按钮：保持 inline 排版，图标按字号缩放并对齐文字基线，
// 避免 inline-flex 在段落中与前后文字错位
defineProps<{
  to?: RouteLocationRaw
  href?: string
  icon?: Component
}>()
defineEmits<{ (e: 'click', ev: MouseEvent): void }>()

const cls = 'text-primary cursor-pointer underline-offset-2 hover:underline'
const iconCls = 'mr-0.5 inline-block size-[1.1em] align-[-0.2em]'
</script>

<template>
  <router-link v-if="to" :to="to" :class="cls"><component :is="icon" v-if="icon" :class="iconCls" /><slot /></router-link>
  <a v-else-if="href" :href="href" target="_blank" rel="noopener" :class="cls"><component :is="icon" v-if="icon" :class="iconCls" /><slot /></a>
  <button v-else type="button" :class="cls" @click="$emit('click', $event)"><component :is="icon" v-if="icon" :class="iconCls" /><slot /></button>
</template>
