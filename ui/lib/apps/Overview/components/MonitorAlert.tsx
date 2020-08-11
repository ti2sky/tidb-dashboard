import React, { useEffect, useState } from 'react'
import { RightOutlined, WarningOutlined } from '@ant-design/icons'
import { Card, AnimatedSkeleton } from '@lib/components'
import client from '@lib/client'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useClientRequest } from '@lib/utils/useClientRequest'
import { Space, Typography } from 'antd'
import { Stack } from 'office-ui-fabric-react/lib/Stack'

export default function MonitorAlert() {
  const { t } = useTranslation()
  const [alertCounter, setAlertCounter] = useState(0)

  const {
    data: amData,
    isLoading: amIsLoading,
  } = useClientRequest((cancelToken) =>
    client.getInstance().getAlertManagerTopology({ cancelToken })
  )
  const {
    data: grafanaData,
    isLoading: grafanaIsLoading,
  } = useClientRequest((cancelToken) =>
    client.getInstance().getGrafanaTopology({ cancelToken })
  )

  const { data: infoData } = useClientRequest((cancelToken) =>
    client.getInstance().getInfo({ cancelToken })
  )

  useEffect(() => {
    if (!amData) {
      return
    }
    async function fetch() {
      let resp = await client
        .getInstance()
        .getAlertManagerCounts(`${amData!.ip}:${amData!.port}`)
      setAlertCounter(resp.data)
    }
    fetch()
  }, [amData])

  const alertInner = (
    <Space>
      <Typography.Text type={alertCounter > 0 ? 'danger' : undefined}>
        {alertCounter === 0
          ? t('overview.monitor_alert.view_zero_alerts')
          : t('overview.monitor_alert.view_alerts', {
              alertCount: alertCounter,
            })}
      </Typography.Text>
      <RightOutlined />
    </Space>
  )

  return (
    <Card title={t('overview.monitor_alert.title')} noMarginLeft>
      <Stack gap={16}>
        <AnimatedSkeleton
          showSkeleton={grafanaIsLoading}
          paragraph={{ rows: 1 }}
        >
          {infoData?.enable_experimental && (
            <Link to={`/metrics`}>
              <Space>
                {t('overview.monitor_alert.view_monitor')}
                <RightOutlined />
              </Space>
            </Link>
          )}
          {!infoData?.enable_experimental && !grafanaData && (
            <Typography.Text type="warning">
              <Space>
                <WarningOutlined />
                {t('overview.monitor_alert.view_monitor_warn')}
              </Space>
            </Typography.Text>
          )}
          {!infoData?.enable_experimental && grafanaData && (
            <a href={`http://${grafanaData.ip}:${grafanaData.port}`}>
              <Space>
                {t('overview.monitor_alert.view_monitor')}
                <RightOutlined />
              </Space>
            </a>
          )}
        </AnimatedSkeleton>
        <AnimatedSkeleton showSkeleton={amIsLoading} paragraph={{ rows: 1 }}>
          {!amData && (
            <Typography.Text type="warning">
              <Space>
                <WarningOutlined />
                {t('overview.monitor_alert.view_alerts_warn')}
              </Space>
            </Typography.Text>
          )}
          {amData && infoData?.enable_experimental && (
            <Link to={`/alerts`}>{alertInner}</Link>
          )}
          {amData && !infoData?.enable_experimental && (
            <a href={`http://${amData.ip}:${amData.port}`}>{alertInner}</a>
          )}
        </AnimatedSkeleton>
        <div>
          <Link to={`/diagnose`}>
            <Space>
              {t('overview.monitor_alert.run_diagnose')}
              <RightOutlined />
            </Space>
          </Link>
        </div>
      </Stack>
    </Card>
  )
}
