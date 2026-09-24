<script setup lang="ts">
// 品牌标识：几何云 + 速度线，底色跟随主题强调色（与 public/favicon.svg 同形）
</script>

<template>
  <svg viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <rect width="64" height="64" rx="15" class="fill-primary" />
    <g transform="translate(2 0)">
      <g fill="#fff">
        <circle cx="27" cy="38" r="6" />
        <circle cx="38" cy="31" r="10" />
        <circle cx="49" cy="37" r="7" />
        <rect x="27" y="37" width="22" height="7" />
      </g>
      <path d="M11 33h8M8 40h9" stroke="#fff" stroke-width="3.5" stroke-linecap="round" />
    </g>
  </svg>
</template>
