<script setup lang="ts">
import { onMounted } from 'vue'
import { Events } from "@wailsio/runtime";

onMounted(() => {
  Events.On('uploadStatus', (uploadStatus: { data: string }) => {
    console.log(uploadStatus.data)
  });
})

</script>

<template>

</template>
