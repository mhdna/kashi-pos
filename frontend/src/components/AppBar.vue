<template>
  <v-app-bar color="grey-darken-3">
    <!-- <template v-slot:prepend> -->
    <!--   <v-app-bar-nav-icon></v-app-bar-nav-icon> -->
    <!-- </template> -->

    <template v-slot:prepend>
      <div class="d-flex flex-column align-last justify-center">
        <v-app-bar-title>KASHI POS Terminal</v-app-bar-title>
        <div class="text-title-small">Branch A</div>
      </div>
    </template>

    <template v-slot:append>
      <!-- <v-btn icon> -->
      <!--   <v-icon>mdi-dots-vertical</v-icon> -->
      <!-- </v-btn> -->
      <v-list-item title="Sandra Adams" subtitle="Salesman">
        <template v-slot:prepend>
          <v-avatar color="purple" size="42">
            <span class="text-headline-small">SA</span>
          </v-avatar>
        </template>
      </v-list-item>
    </template>
  </v-app-bar>
</template>