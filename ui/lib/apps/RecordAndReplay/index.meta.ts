import {PlayCircleOutlined} from '@ant-design/icons'
import translations from './translations'

export default {
  id: 'record_and_replay',
  routerPrefix: '/record_and_replay',
  icon: PlayCircleOutlined,
  translations,
  reactRoot: () => import(/* webpackChunkName: "record_and_replay" */ '.'),
}
