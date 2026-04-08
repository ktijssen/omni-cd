<template>
  <div class="auth-wrap">
    <div class="auth-logo">
      <span v-html="omniLogoSVG" style="width:28px;height:28px;flex-shrink:0;display:flex;align-items:center;justify-content:center;"></span>
      <span class="auth-logo-text">Omni <span>CD</span></span>
    </div>
    <div class="auth-card">
      <div style="font-size:40px;text-align:center;margin-bottom:16px;">🚫</div>
      <div class="auth-title" style="text-align:center;">Access Denied</div>
      <div class="auth-sub" style="text-align:center;margin-bottom:24px;">
        Your account does not have permission to access Omni CD.<br>
        Contact your administrator to request access.
      </div>
      <a href="/logout" class="auth-btn" style="display:block;text-align:center;text-decoration:none;">Sign out</a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { omniLogoSVG } from '@/assets/icons'
</script>
