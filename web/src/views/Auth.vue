<template>
  <div class="auth">
    <v-dialog v-model="loginHelpDialog" max-width="600">
      <v-card>
        <v-card-title>
          {{ $t('howToFixSigninIssues') }}
          <v-spacer></v-spacer>
          <v-btn icon @click="loginHelpDialog = false">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </v-card-title>
        <v-card-text>
          <p class="text-body-1">
            {{ $t('firstlyYouNeedAccessToTheServerWhereSemaphoreRunni') }}
          </p>
          <p class="text-body-1">
            {{ $t('executeTheFollowingCommandOnTheServerToSeeExisting') }}
          </p>
          <v-alert
            dense
            text
            color="info"
            style="font-family: monospace;"
          >
            {{ $t('semaphoreUserList') }}
          </v-alert>
          <p class="text-body-1">
            {{ $t('youCanChangePasswordOfExistingUser') }}
          </p>
          <v-alert
            dense
            text
            color="info"
            style="font-family: monospace;"
          >
            {{
              $t('semaphoreUserChangebyloginLoginUser123Password', {
                makePasswordExample:
                  makePasswordExample()
              })
            }}
          </v-alert>
          <p class="text-body-1">
            {{ $t('orCreateNewAdminUser') }}
          </p>
          <v-alert
            dense
            text
            color="info"
            style="font-family: monospace;"
          >
            semaphore user add --admin --login user123 --name User123
            --email user123@example.com --password {{ makePasswordExample() }}
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer/>
          <v-btn
            color="blue darken-1"
            text
            @click="loginHelpDialog = false"
          >
            {{ $t('close2') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-container
      fluid
      fill-height
      align-center
      justify-center
      class="pa-0"
    >
      <v-form
        ref="signInForm"
        lazy-validation
        v-model="signInFormValid"
        style="width: 300px;"
      >

        <v-img
          width="80"
          height="80"
          transition="0"
          src="favicon.png"
          style="margin: auto;"
          class="mb-4"
        />

        <h3 class="text-center mb-8">{{ $t('semaphore') }}</h3>

        <v-alert
          :value="signInError != null"
          color="error"
          style="margin-bottom: 20px;"
        >{{ signInError }}
        </v-alert>

        <v-text-field
          v-model="username"
          v-bind:label='$t("username")'
          :rules="[v => !!v || $t('username_required')]"
          required
          :disabled="signInProcess"
          v-if="loginWithPassword"
        ></v-text-field>

        <v-text-field
          v-model="password"
          :label="$t('password')"
          :rules="[v => !!v || $t('password_required')]"
          type="password"
          required
          :disabled="signInProcess"
          @keyup.enter.native="signIn"
          style="margin-bottom: 20px;"
          v-if="loginWithPassword"
        ></v-text-field>

        <v-btn
          color="primary"
          @click="signIn"
          :disabled="signInProcess"
          block
          v-if="loginWithPassword"
        >
          {{ $t('signIn') }}
        </v-btn>

        <v-btn
          v-for="provider in oidcProviders"
          :color="provider.color || 'secondary'"
          dark
          class="mt-2"
          @click="oidcSignIn(provider.id)"
          block
          :key="provider.id"
        >
          <v-icon
            left
            dark
            v-if="provider.icon"
          >
            mdi-{{ provider.icon }}
          </v-icon>

          {{ provider.name }}
        </v-btn>

        <div class="text-center mt-6" v-if="loginWithPassword">
          <a @click="loginHelpDialog = true">{{ $t('dontHaveAccountOrCantSignIn') }}</a>
        </div>
      </v-form>
    </v-container>
  </div>
</template>
<style lang="scss">
.auth {
  height: 100vh;
}
</style>
<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';

const prefersDarkMode = window.matchMedia('(prefers-color-scheme: dark)');

export default {
  data() {
    return {
      signInFormValid: false,
      signInError: null,
      signInProcess: false,

      password: null,
      username: null,

      loginHelpDialog: null,

      oidcProviders: [],
      loginWithPassword: null,

      darkMode: false,
    };
  },

  watch: {
    darkMode(val) {
      this.$vuetify.theme.dark = val;
    },
  },

  async created() {
    if (this.isAuthenticated()) {
      document.location = document.baseURI;
    }
    await axios({
      method: 'get',
      url: '/api/auth/login',
      responseType: 'json',
    }).then((resp) => {
      this.oidcProviders = resp.data.oidc_providers;
      this.loginWithPassword = resp.data.login_with_password;
    });

    const isDarkMode = localStorage.getItem('darkMode');
    if (isDarkMode !== null) {
      this.darkMode = isDarkMode === '1';
    } else {
      prefersDarkMode.addEventListener('change', (e) => {
        this.darkMode = e.matches;
      });

      if (prefersDarkMode.matches && localStorage.getItem('darkMode') !== '0') {
        this.darkMode = true;
      }
    }
  },

  methods: {
    makePasswordExample() {
      let pwd = '';
      const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
      const charactersLength = characters.length;
      for (let i = 0; i < 10; i += 1) {
        pwd += characters.charAt(Math.floor(Math.random() * charactersLength));
      }
      return pwd;
    },

    isAuthenticated() {
      return document.cookie.includes('semaphore=');
    },

    async signIn() {
      this.signInError = null;

      if (!this.$refs.signInForm.validate()) {
        return;
      }

      this.signInProcess = true;
      try {
        await axios({
          method: 'post',
          url: '/api/auth/login',
          responseType: 'json',
          data: {
            auth: this.username,
            password: this.password,
          },
        });
        document.location = document.baseURI + window.location.search;
      } catch (err) {
        if (err.response.status === 401) {
          this.signInError = this.$t('incorrectUsrPwd');
        } else {
          this.signInError = getErrorMessage(err);
        }
      } finally {
        this.signInProcess = false;
      }
    },

    async oidcSignIn(provider) {
      document.location = `/api/auth/oidc/${provider}/login${window.location.search}`;
    },
  },
};
</script>
