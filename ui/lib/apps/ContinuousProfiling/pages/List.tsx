import { Badge, Tooltip, Space, Drawer, Result, Button, Alert } from 'antd'
import { ScrollablePane } from 'office-ui-fabric-react/lib/ScrollablePane'
import React, { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { usePersistFn, useSessionStorageState } from 'ahooks'
import {
  LoadingOutlined,
  ReloadOutlined,
  SettingOutlined,
} from '@ant-design/icons'

import client, { ErrorStrategy } from '@lib/client'
import {
  Card,
  CardTable,
  Toolbar,
  TimeRangeSelector,
  TimeRange,
  calcTimeRange,
} from '@lib/components'
import DateTime from '@lib/components/DateTime'
import openLink from '@lib/utils/openLink'
import { useClientRequest } from '@lib/utils/useClientRequest'
import { InstanceKindName } from '@lib/utils/instanceTable'

import ConProfSettingForm from './ConProfSettingForm'

import styles from './List.module.less'

export default function Page() {
  const {
    data: historyTable,
    isLoading: listLoading,
    error: historyError,
    sendRequest: reloadGroupProfiles,
  } = useClientRequest(() => {
    const [beginTime, endTime] = calcTimeRange(timeRange)
    return client
      .getInstance()
      .continuousProfilingGroupProfilesGet(beginTime, endTime, {
        errorStrategy: ErrorStrategy.Custom,
      })
  })
  const { t } = useTranslation()
  const navigate = useNavigate()

  const handleRowClick = usePersistFn(
    (rec, _idx, ev: React.MouseEvent<HTMLElement>) => {
      openLink(`/continuous_profiling/detail?ts=${rec.ts}`, ev, navigate)
    }
  )

  const historyTableColumns = useMemo(
    () => [
      {
        name: t('continuous_profiling.list.table.columns.targets'),
        key: 'targets',
        minWidth: 150,
        maxWidth: 250,
        onRender: (rec) => {
          const { tikv, tidb, pd, tiflash } = rec.component_num
          const s = `${tikv} ${InstanceKindName['tikv']}, ${tidb} ${InstanceKindName['tidb']}, ${pd} ${InstanceKindName['pd']}, ${tiflash} ${InstanceKindName['tiflash']}`
          return <span>{s}</span>
        },
      },
      {
        name: t('continuous_profiling.list.table.columns.status'),
        key: 'status',
        minWidth: 100,
        maxWidth: 150,
        onRender: (rec) => {
          if (rec.state === 'failed') {
            // all failed
            return (
              <Badge
                status="error"
                text={t('continuous_profiling.list.table.status.failed')}
              />
            )
          } else if (rec.state === 'success') {
            // all success
            return (
              <Badge
                status="success"
                text={t('continuous_profiling.list.table.status.finished')}
              />
            )
          } else {
            // partial failed
            return (
              <Badge
                status="warning"
                text={t(
                  'continuous_profiling.list.table.status.partial_finished'
                )}
              />
            )
          }
        },
      },
      {
        name: t('continuous_profiling.list.table.columns.start_at'),
        key: 'ts',
        minWidth: 160,
        maxWidth: 220,
        onRender: (rec) => {
          return <DateTime.Long unixTimestampMs={rec.ts * 1000} />
        },
      },
      {
        name: t('continuous_profiling.list.table.columns.duration'),
        key: 'duration',
        minWidth: 100,
        maxWidth: 150,
        fieldName: 'profile_duration_secs',
      },
    ],
    [t]
  )

  const [timeRange, setTimeRange] = useSessionStorageState<
    TimeRange | undefined
  >('conprof.timerange', undefined)

  function onTimeRangeChange(v: TimeRange) {
    setTimeRange(v)
    setTimeout(() => {
      reloadGroupProfiles()
    }, 0)
  }

  const [showSettings, setShowSettings] = useState(false)

  const { data: ngMonitoringConfig, sendRequest: reloadConfig } =
    useClientRequest((reqConfig) =>
      client.getInstance().continuousProfilingConfigGet(reqConfig)
    )
  const conprofIsDisabled = useMemo(
    () => ngMonitoringConfig?.continuous_profiling?.enable === false,
    [ngMonitoringConfig]
  )

  function refresh() {
    reloadConfig()
    reloadGroupProfiles()
  }

  return (
    <div className={styles.list_container}>
      <Card>
        <Toolbar className={styles.list_toolbar}>
          <Space>
            <TimeRangeSelector value={timeRange} onChange={onTimeRangeChange} />
          </Space>
          <Space>
            <Tooltip
              title={t('continuous_profiling.list.toolbar.refresh')}
              placement="bottom"
            >
              {listLoading ? (
                <LoadingOutlined />
              ) : (
                <ReloadOutlined onClick={refresh} />
              )}
            </Tooltip>
            <Tooltip
              title={t('continuous_profiling.list.toolbar.settings')}
              placement="bottom"
            >
              <SettingOutlined onClick={() => setShowSettings(true)} />
            </Tooltip>
          </Space>
        </Toolbar>
      </Card>

      {conprofIsDisabled && historyTable && historyTable.length > 0 && (
        <div className={styles.alert_container}>
          <Alert
            message={t('continuous_profiling.settings.disabled_with_history')}
            type="info"
            showIcon
          />
        </div>
      )}

      {conprofIsDisabled && historyTable?.length === 0 ? (
        <Result
          title={t('continuous_profiling.settings.disabled_result.title')}
          subTitle={t(
            'continuous_profiling.settings.disabled_result.sub_title'
          )}
          extra={
            <Button type="primary" onClick={() => setShowSettings(true)}>
              {t('continuous_profiling.settings.open_settings')}
            </Button>
          }
        />
      ) : (
        <div style={{ height: '100%', position: 'relative' }}>
          <ScrollablePane>
            <CardTable
              cardNoMarginTop
              loading={listLoading}
              items={historyTable || []}
              columns={historyTableColumns}
              errors={[historyError]}
              onRowClicked={handleRowClick}
            />
          </ScrollablePane>
        </div>
      )}

      <Drawer
        title={t('continuous_profiling.settings.title')}
        width={300}
        closable={true}
        visible={showSettings}
        onClose={() => setShowSettings(false)}
        destroyOnClose={true}
      >
        <ConProfSettingForm
          onClose={() => setShowSettings(false)}
          onConfigUpdated={reloadConfig}
        />
      </Drawer>
    </div>
  )
}
