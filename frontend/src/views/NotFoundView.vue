<template>
  <div class="container">
    <div style="display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;padding:80px 16px;text-align:center;">
      <div style="font-size:40px;line-height:1;">🧭</div>
      <h1 style="font-size:20px;font-weight:600;color:#fff;letter-spacing:-0.3px;margin:0;">Page not found</h1>
      <p style="color:#7d7d85;font-size:13px;max-width:360px;margin:0;">
        <code style="color:#c4c4c9;">{{ attemptedPath }}</code> doesn't exist or you don't have access to it.
      </p>
      <RouterLink to="/clusters" class="btn-omni" style="text-decoration:none;margin-top:8px;">Back to Clusters</RouterLink>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

const route = useRoute()
const attemptedPath = computed(() => route.fullPath)
</script>
