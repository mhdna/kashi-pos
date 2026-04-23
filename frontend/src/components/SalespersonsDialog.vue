<template>
    <v-dialog v-model="dialog" :close-on-content-click="false" location="bottom" max-width="800">
        <template v-slot:activator="{ props }">
            <v-btn v-bind="props" class="py-6 px-0 me-4">
                <v-list-item title="Sandra Adams" subtitle="Salesperson">
                    <template v-slot:prepend>
                        <v-avatar color="purple" size="42">
                            <span class="text-headline-small">SA</span>
                        </v-avatar>
                    </template>
                </v-list-item>
            </v-btn>
        </template>
        <v-card max-width="800" max-height="550" class="px-2 pb-2">
            <v-card-title>
                Salespersons
            </v-card-title>
            <div class="d-flex flex-wrap">
                <v-card v-for="p in salespersons" min-width="250" min-height="150" link
                    class="ma-1 d-flex flex-column justify-center align-center">
                    <initials-avatar :firstName="p.firstName" :lastName="p.lastName" :color="p.color" />
                    <div class="mt-5 text-headline-small">
                        {{ p.firstName }}
                        {{ p.lastName }}
                    </div>
                </v-card>
            </div>
        </v-card>
    </v-dialog>
</template>
<script setup>
import { ref } from 'vue'
import InitialsAvatar from './InitialsAvatar.vue';

const dialog = ref(false)

const salespersons = ref([
    { id: 1, firstName: "Sandra", lastName: "Adams", color: "yellow" },
    { id: 2, firstName: "Khanda", lastName: "Adams", color: "blue" },
    { id: 3, firstName: "Uganda", lastName: "Adams", color: "red" },
    { id: 4, firstName: "Sinda", lastName: "Adams", color: "pink" },
    { id: 5, firstName: "Sinda", lastName: "Adams", color: "purple" },
    { id: 6, firstName: "Sinda", lastName: "Adams", color: "green" },
    { id: 6, firstName: "Sinda", lastName: "Adams", color: "teal" },
    { id: 7, firstName: "Sinda", lastName: "Adams", color: "cyan" },
])
</script>