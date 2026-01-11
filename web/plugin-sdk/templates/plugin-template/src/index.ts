import type { PluginDefinition } from '@gameap/plugin-sdk';
import PluginPage from './components/PluginPage.vue';
import DashboardWidget from './components/DashboardWidget.vue';
import ServerTab from './components/ServerTab.vue';

export const myPlugin: PluginDefinition = {
    id: 'my-plugin',
    name: 'My Plugin',
    version: '1.0.0',
    apiVersion: '1.0',
    description: 'A sample GameAP plugin',
    author: 'Your Name',

    translations: {
        en: {
            // Plugin page translations
            'title': 'My Plugin Page',
            'welcome': 'Welcome to your plugin! This is the main plugin page.',
            'user_info': 'User Information',
            'logged_in_as': 'Logged in as',
            'admin': 'Admin',
            'yes': 'Yes',
            'no': 'No',
            // Menu and navigation
            'menu_item': 'My Plugin',
            'server_tab': 'My Tab',
            'dashboard_widget': 'My Widget',
            // Server tab translations
            'tab_title': 'My Plugin Tab',
            'tab_description': 'This tab was added by your plugin. It demonstrates access to server data.',
            'server_id': 'Server ID',
            'server_name': 'Server Name',
            'game': 'Game',
            'address': 'Address',
            'status': 'Status',
            'unknown': 'Unknown',
            'running': 'Running',
            'disabled': 'Disabled',
            'stopped': 'Stopped',
            // Dashboard widget translations
            'widget_title': 'My Plugin Widget',
            'widget_description': 'This widget was added by your plugin. It appears on the dashboard.',
            'admin_notice': 'You are viewing this as an admin.',
        },
        ru: {
            // Plugin page translations
            'title': 'Страница плагина',
            'welcome': 'Добро пожаловать в ваш плагин! Это главная страница плагина.',
            'user_info': 'Информация о пользователе',
            'logged_in_as': 'Вы вошли как',
            'admin': 'Администратор',
            'yes': 'Да',
            'no': 'Нет',
            // Menu and navigation
            'menu_item': 'Мой плагин',
            'server_tab': 'Моя вкладка',
            'dashboard_widget': 'Мой виджет',
            // Server tab translations
            'tab_title': 'Вкладка плагина',
            'tab_description': 'Эта вкладка добавлена вашим плагином. Она демонстрирует доступ к данным сервера.',
            'server_id': 'ID сервера',
            'server_name': 'Имя сервера',
            'game': 'Игра',
            'address': 'Адрес',
            'status': 'Статус',
            'unknown': 'Неизвестно',
            'running': 'Запущен',
            'disabled': 'Отключён',
            'stopped': 'Остановлен',
            // Dashboard widget translations
            'widget_title': 'Виджет плагина',
            'widget_description': 'Этот виджет добавлен вашим плагином. Он отображается на главной странице.',
            'admin_notice': 'Вы просматриваете это как администратор.',
        },
    },

    routes: [
        {
            path: '/',
            name: 'index',
            component: PluginPage,
            meta: { title: 'My Plugin' },
        },
    ],

    menuItems: [
        {
            section: 'servers',
            icon: 'fas fa-puzzle-piece',
            text: '@:menu_item',
            route: { name: 'index' },
            order: 100,
        },
    ],

    homeButtons: [
        {
            name: '@:title',
            icon: 'fas fa-puzzle-piece',
            route: { name: 'index' },
            order: 100,
        },
    ],

    slots: {
        'dashboard-widgets': [
            {
                component: DashboardWidget,
                order: 50,
                label: '@:dashboard_widget',
            },
        ],
        'server-tabs': [
            {
                component: ServerTab,
                order: 100,
                label: '@:server_tab',
                icon: 'fas fa-puzzle-piece',
                name: 'my-tab',
            },
        ],
    },
};
