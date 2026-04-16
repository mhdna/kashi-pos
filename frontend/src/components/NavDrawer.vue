<template>
  <v-navigation-drawer permanent width="140">
    <v-divider></v-divider>
    <v-list nav>
      <v-list-item value="invoice">
        <div height="140" flat hover class="d-flex flex-column justify-center align-center" density="compact">
          <v-icon size="40">mdi-invoice</v-icon>
          <h3>Invoice</h3>
        </div>
      </v-list-item>
      <v-list-item value="inventory">
        <div height="140" flat hover class="d-flex flex-column justify-center align-center" density="compact">
          <v-icon size="40">mdi-chart-bell-curve</v-icon>
          <h3>Inventory</h3>
        </div>
      </v-list-item>
      <v-list-item value="report">
        <div height="140" flat hover class="d-flex flex-column justify-center align-center" density="compact">
          <v-icon size="40">mdi-chart-box</v-icon>
          <h3>X-Report</h3>
        </div>
      </v-list-item>
      <!-- <v-list-item  value="inventory" title="Inventory"> -->
      <!--   <template v-slot:prepend> -->
      <!--       <v-icon size="65">mdi-cart</v-icon> -->
      <!--   </template> -->
      <!-- </v-list-item> -->
    </v-list>
  </v-navigation-drawer>
</template>