<template>
    <v-dialog v-model="dialog" max-height="600" max-width="600">
        <v-card title="clients">
            <invoices-table />
        </v-card>
    </v-dialog>
    <v-sheet class="d-flex flex-column w-100" style="height: 91vh;">
        <v-card class="ma-0 d-flex justify-center text-title-medium my-2 w-100 flex-column" color="gray"
            style="flex:3; " rounded="false" title="Client" prepend-icon="mdi-account">
            <v-card-text class="d-flex flex-column align-center">
                <v-card color="surface-light" elevation="0" class="text-title-large px-3 py-1 ma-4 text-center"
                    rounded="lg" width="100%" @click="dialog = true">
                    <v-avatar color="yellow-darken-3" size="42">
                        <span class="text-headline-small">TP</span>
                    </v-avatar>
                    <div>
                        Test Person
                    </div>
                    <div class="text-subtitle text-title-medium">08832/2348432</div>
                </v-card>
                <div class="text-subtitle text-title-medium text-center">
                    <div>Gold Tier</div>
                    <div>Has: 200 Points</div>
                    <div>Earns: 47 Points</div>
                </div>
            </v-card-text>
        </v-card>
        <!-- <v-card width="100%" class="d-flex justify-center align-center text-title-medium my-2 flex-grow-1" color="gray" -->
        <!--     title="Client" prepend-icon="mdi-circle"> -->
        <!--     15$ -->
        <!-- </v-card> -->
        <!-- <v-btn variant="tonal" width="100%" -->
        <!--     class="d-flex justify-center align-center text-title-medium my-2 flex-grow-1" color="green-darken-4" -->
        <!--     text="Deposit" prepend-icon="mdi-percent"> -->
        <!-- </v-btn> -->
        <v-btn variant="tonal" width="100%"
            class="d-flex justify-center align-center text-title-medium my-2 flex-grow-1" color="gray" text="Void"
            prepend-icon="mdi-circle">
        </v-btn>
        <v-btn variant="tonal" width="100%"
            class="d-flex justify-center align-center text-title-medium my-2 flex-grow-1" color="yellow-darken-4"
            text="Hold" prepend-icon="mdi-pause">
        </v-btn>
        <v-btn variant="tonal" width="100%"
            class="d-flex justify-center align-center text-title-medium my-2 flex-grow-1" color="green-darken-4"
            text="Check Out" prepend-icon="mdi-cash">
        </v-btn>
    </v-sheet>
</template>

<script setup>
import { ref } from 'vue';
import InvoicesTable from './InvoicesTable.vue';
const dialog = ref(false)
</script>