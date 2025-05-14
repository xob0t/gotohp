<script setup lang="ts">
import Upoad from './components/Upoad.vue'
import { Browser } from "@wailsio/runtime";
import './index.css'

import { useColorMode } from '@vueuse/core'

useColorMode().value = "dark"

</script>

<template>
  <main class="">
    <h1>
      Drop files to upload them
    </h1>
    <Upoad />
  </main>
</template>
