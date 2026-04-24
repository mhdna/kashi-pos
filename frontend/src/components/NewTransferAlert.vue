<template>
    <v-list-item>
        <div class="d-flex align-center">
            <v-icon class="flash-alert me-2" icon="mdi-information" size="32"></v-icon>
            <v-card type="info" color="transparent" class="text-yellow" max-width="280">
                <div class="marquee-wrap">
                    <div class="m">
                        <span>New Transfer Arrived. Please Check the transfers page for confirmation.</span>
                        <span>New Transfer Arrived. Please Check the transfers page for confirmation.</span>
                    </div>
                </div>
            </v-card>
        </div>
    </v-list-item>
</template>

<style scoped>
.flash-alert {
    animation: flash 1.2s ease-in-out infinite;
}

@keyframes flash {

    0%,
    100% {
        opacity: 1;
    }

    50% {
        opacity: 0.3;
    }
}

.marquee-wrap {
    overflow: hidden;
    width: 100%;
}

.m {
    display: flex;
    /* spans sit side by side */
    width: max-content;
    /* expands to fit both spans */
    animation: marquee 6s infinite linear;

    span {
        white-space: nowrap;
        font-size: 18px;
        padding-right: 3rem;
        /* gap between end and start */
    }
}

@keyframes marquee {
    0% {
        transform: translateX(0);
    }

    100% {
        transform: translateX(-50%);
    }
}
</style>
