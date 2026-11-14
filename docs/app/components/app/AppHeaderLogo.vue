<script setup lang="ts">
const appConfig = useAppConfig()
const { contextMenuItems } = useLogoAssets()
</script>

<template>
  <UContextMenu :items="contextMenuItems">
    <div class="flex items-center py-1">
      <!-- Light mode header logo (displayed on light backgrounds) -->
      <img
        src="/2.png"
        :alt="appConfig.header?.logo?.alt || appConfig.header?.title || 'Forge'"
        class="h-12 sm:h-16 md:h-16 w-auto max-h-16 block dark:hidden object-contain shrink-0 transition-all duration-200"
      />
      <!-- Dark mode header logo (displayed on dark backgrounds) -->
      <img
        src="/1.png"
        :alt="appConfig.header?.logo?.alt || appConfig.header?.title || 'Forge'"
        class="h-12 sm:h-16 md:h-16 w-auto max-h-16 hidden dark:block object-contain shrink-0 transition-all duration-200"
      />
    </div>
  </UContextMenu>
</template>
