<template>
    <v-avatar :color="props.color" size="60">
        <span style="letter-spacing: 1px; font-size: 25px;">{{ firstName[0] }}{{ lastName[0]
        }}</span>
    </v-avatar>
</template>

<script setup>
import { defineProps } from 'vue';

const props = defineProps({
    color: {
        type: String,
        required: true
    },
    firstName: {
        type: String,
        required: true
    },
    lastName: {
        type: String,
        required: true
    }
})
</script>