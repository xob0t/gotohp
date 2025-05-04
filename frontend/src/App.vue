<script setup lang="ts">
import HelloWorld from './components/HelloWorld.vue'
import { Toaster } from '@/components/ui/sonner'
import { Browser } from "@wailsio/runtime";
import './index.css'

import { useColorMode } from '@vueuse/core'

import WindowTitlebar from "./wails-controls/WindowTitlebar.vue"

useColorMode().value = "dark"

</script>

<template>
  <div>
    <WindowTitlebar class="absolute top-0 left-0 w-full">
      <span class="ml-4 font-semibold">gotohs</span>
    </WindowTitlebar>
  </div>

  <Toaster />
  <main class="h-[calc(100vh-2rem)] w-full bottom-0 mt-8 p-20">
    <div class="container" style="--wails-draggable: drag">
      <div class="flex justify-center">
        <a @click="Browser.OpenURL('https://wails.io')" class="cursor-pointer">
          <img src="/wails.png" class="h-36 p-2 hover:drop-shadow-2xl" alt="Wails logo" />
        </a>
        <a @click="Browser.OpenURL('https://vuejs.org/')" class="cursor-pointer">
          <img src="/vue.svg" class="h-36 p-2 hover:drop-shadow-2xl" alt="Vue logo" />
        </a>
      </div>
      <HelloWorld msg="Wails + Vue" />
    </div>
  </main>
</template>
