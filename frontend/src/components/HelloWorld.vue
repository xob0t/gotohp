<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GreetService } from "../../bindings/backend";
import { Events } from "@wailsio/runtime";
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'


defineProps<{ msg: string }>()

const name = ref('')
const result = ref('Please enter your name below 👇')
const time = ref('Listening for Time event...')

const doGreet = () => {
  let localName = name.value;
  if (!localName) {
    localName = 'anonymous';
  }
  GreetService.Greet(localName).then((resultValue: string) => {

    toast("Greetings!", {
      description: resultValue,
      action: {
        label: 'Undo',
        onClick: () => console.log('Undo'),
      },
    })

    result.value = resultValue;
  }).catch((err: Error) => {
    console.log(err);
  });
}

onMounted(() => {
  Events.On('time', (timeValue: { data: string }) => {
    time.value = timeValue.data;
  });
})

</script>

<template>
  <div class="items-center flex flex-col gap-5">

    <h1 class="text-4xl">{{ msg }}</h1>

    <div class="result">{{ result }}</div>
    <div class="flex items-center">
      <Input v-model="name" type="text" autocomplete="off" placeholder="Enter your name" />
      <Button @click="doGreet" class="cursor-pointer">Greet</Button>
    </div>
  </div>

  <Alert class="mt-30">
    <AlertTitle>Heads up!</AlertTitle>
    <AlertDescription>
      <p>Click on the Wails logo to learn more</p>
      <p>{{ time }}</p>
    </AlertDescription>
  </Alert>

</template>
