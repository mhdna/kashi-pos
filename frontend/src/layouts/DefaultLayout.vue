<template>
    <v-app>
        <AppBar />
        <NavDrawer />
        <v-main>
            <v-container fluid class="pa-0" style="height: 91vh; overflow: hidden;">
                <router-view />
            </v-container>
        </v-main>
    </v-app>
</template>

<script setup>
import AppBar from '@/components/AppBar.vue'
import NavDrawer from '@/components/NavDrawer.vue'
</script>