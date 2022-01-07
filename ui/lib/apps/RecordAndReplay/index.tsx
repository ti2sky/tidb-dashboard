import React, {useCallback, useMemo, useState} from 'react'
import {CardTable, Head} from '@lib/components'
import {Badge, Button, Space} from 'antd'
import {CaretRightOutlined,} from '@ant-design/icons'
import client, {RecordandreplayTask} from "@lib/client";
import {useTranslation} from "react-i18next";
import {ScrollablePane} from "office-ui-fabric-react/lib/ScrollablePane";
import {useClientRequestWithPolling} from "@lib/utils/useClientRequest";

enum taskState {
    Recording,
    FinishRecording,
    Replaying,
    FinishReplaying,
    Error,
}

function filenameRender({name}: RecordandreplayTask) {
    return (
        <span>{name}</span>
    )
}

function startTimeRender(task: RecordandreplayTask) {
    return (
        <span>{task?.start_time}</span>
    )
}

function endTimeRender(task: RecordandreplayTask) {
    if (task.state == taskState.Recording) {
        return
    } else {
        return (
            <span>{task?.end_time}</span>
        )
    }
}

function updateTasks(tasks: Array<RecordandreplayTask> | undefined) {
    if (!tasks) {
        return tasks
    }
    tasks.forEach(t => {
        // if (t.state === taskState.Recording) {
        //     setRecording(true)
        // } else if (t.state === taskState.Replaying) {
        //     setReplaying(true)
        // }
    })
    return tasks
}

function isFinished(task: RecordandreplayTask) {
    return task === taskState.FinishRecording || task === taskState.Error
}

function App() {
    const {t} = useTranslation()
    const [isRecording, setRecording] = useState(false)
    const [isReplaying, setReplaying] = useState(false)

    // const [tasks, setTasks] = useState<RecordandreplayTask[]>([])
    const [selectedRowKeys, setRowKeys] = useState<string[]>([])

    const {
        data: respData,
        isLoading,
        error,
    } = useClientRequestWithPolling(
        () => client.getInstance().recordandreplayTasksGet(),
    )

    const data = useMemo(() => updateTasks(respData), [respData])

    const handleRecord = useCallback(async () => {
        const startTime = Math.floor(Date.now() / 1000)
        const resp = await client.getInstance().recordandreplayTasksRecordPost({
            name: `test-${startTime}`,
            time: startTime,
        })
        if (resp.status == 200) {
            setRecording(true)
        }
    }, [])

    const handleStopRecord = useCallback(async (taskID: string) => {
        const resp = await client.getInstance().recordandreplayTasksIdStopRecordGet(taskID)
        if (resp.status == 200) {
            setRecording(false)
        }
    }, [])

    const handleDelete = useCallback(async (taskID: string) => {
        const resp = await client.getInstance().recordandreplayTasksIdDelete(taskID)
        if (resp.status == 200) {
            console.log("handle delete")
        } else {
            console.log(resp.status)
        }
    }, [])

    async function handleReplay(taskID: string) {
        setReplaying(true)
        await client.getInstance().recordandreplayTasksIdStartReplayGet(taskID)
        setReplaying(false)
    }

    function stateRender({state}: RecordandreplayTask) {
        switch (state) {
            case 0:
                return (
                    <Badge status="processing" text={t('record_and_replay.recording')}/>
                )
            case 1:
                return (
                    <Badge status="success" text={t('record_and_replay.record_finished')}/>
                )
            case 2:
                return (
                    <Badge status="processing" text={t('record_and_replay.replaying')}/>
                )
            case 3:
                return (
                    <Badge status="success" text={t('record_and_replay.replay_finished')}/>
                )
            case 4:
                return (
                    <Badge status="processing" text={t('record_and_replay.task_error')}/>
                )
            default:
                return
        }
    }

    function actionRender(task: RecordandreplayTask) {
        if (task.id === null) {
            return
        }
        switch (task.state) {
            case taskState.Error:
                return
            case taskState.Recording:
                return (
                    <Button
                        type="primary"
                        // icon={<CaretRightOutlined/>}
                        onClick={() => handleStopRecord(task.id as string)}
                        // disabled={isRecording}
                    >
                        {t('record_and_replay.record_stop')}
                    </Button>
                )
            case taskState.FinishRecording:
                return (
                    <Space>
                        <Button
                            type="primary"
                            // icon={<CaretRightOutlined/>}
                            onClick={() => handleReplay(task.id as string)}
                            disabled={isReplaying}
                        >
                            {t('record_and_replay.replay_start')}
                        </Button>
                        <Button
                            danger
                            type="primary"
                            // icon={<CaretRightOutlined/>}
                            onClick={() => handleDelete(task.id as string)}
                            disabled={isReplaying || isRecording}
                        >
                            {t('record_and_replay.delete')}
                        </Button>
                    </Space>
                )
            case taskState.FinishReplaying:
                return (
                    <Space>
                        <Button
                            type="primary"
                            // icon={<CaretRightOutlined/>}
                            onClick={() => handleReplay(task.id as string)}
                            disabled={isReplaying}
                        >
                            {t('record_and_replay.replay_start')}
                        </Button>
                        <Button
                            danger
                            type="primary"
                            // icon={<CaretRightOutlined/>}
                            onClick={() => handleDelete(task.id as string)}
                            disabled={isReplaying || isRecording}
                        >
                            {t('record_and_replay.delete')}
                        </Button>
                    </Space>
                )
        }
    }

    const columns = [
        {
            name: t('record_and_replay.filename'),
            key: 'filename',
            minWidth: 100,
            maxWidth: 300,
            onRender: filenameRender,
        },
        {
            name: t('record_and_replay.start_time'),
            key: 'start_time',
            minWidth: 100,
            maxWidth: 300,
            onRender: startTimeRender,
        },
        {
            name: t('record_and_replay.end_time'),
            key: 'end_time',
            minWidth: 100,
            maxWidth: 300,
            onRender: endTimeRender,
        },
        {
            name: t('record_and_replay.status'),
            key: 'state',
            minWidth: 100,
            maxWidth: 200,
            onRender: stateRender,
        },
        {
            name: t('record_and_replay.action'),
            key: 'action',
            minWidth: 100,
            maxWidth: 300,
            onRender: actionRender,
        },
    ]

    return (
        <div style={{height: '100vh', display: 'flex', flexDirection: 'column'}}>
            <Head
                title={t('record_and_replay.nav_title')}
                titleExtra={
                    <Space>
                        <Button
                            type="primary"
                            icon={<CaretRightOutlined/>}
                            onClick={handleRecord}
                            disabled={isRecording || isReplaying}
                        >
                            {t('record_and_replay.record_start')}
                        </Button>
                    </Space>
                }
            />
            <div style={{height: '100%', position: 'relative'}}>
                <ScrollablePane>
                    <CardTable
                        cardNoMarginTop
                        columns={columns}
                        items={data || []}
                        loading={isLoading}
                        errors={[error]}
                        hideLoadingWhenNotEmpty
                        // selection={rowSelection}
                        // selectionMode={SelectionMode.multiple}
                    />
                </ScrollablePane>
            </div>
        </div>
    )
}

export default App
