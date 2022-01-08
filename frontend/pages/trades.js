import React, {useEffect, useState} from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Paper from '@material-ui/core/Paper';
import {queryTrades} from '../src/api/bbgo';
import {DataGrid} from '@material-ui/data-grid';
import DashboardLayout from '../layouts/DashboardLayout';

const columns = [
    {field: 'gid', headerName: 'GID', width: 80, type: 'number'},
    {field: 'exchange', headerName: 'Exchange'},
    {field: 'symbol', headerName: 'Symbol'},
    {field: 'side', headerName: 'Side', width: 90},
    {field: 'price', headerName: 'Price', type: 'number', width: 120},
    {field: 'quantity', headerName: 'Quantity', type: 'number'},
    {field: 'isMargin', headerName: 'Margin'},
    {field: 'isIsolated', headerName: 'Isolated'},
    {field: 'tradedAt', headerName: 'Trade Time', width: 200},
];

const useStyles = makeStyles((theme) => ({
    paper: {
        margin: theme.spacing(2),
        padding: theme.spacing(2),
    },
    dataGridContainer: {
        display: 'flex',
        height: 'calc(100vh - 64px - 120px)',
    }
}));

export default function Trades() {
    const classes = useStyles();

    const [trades, setTrades] = useState([])

    useEffect(() => {
        queryTrades({}, (trades) => {
            setTrades(trades.map((o) => {
                o.id = o.gid;
                return o
            }))
        })
    }, [])

    return (
        <DashboardLayout>
            <Paper className={classes.paper}>
                <Typography variant="h4" gutterBottom>
                    Trades
                </Typography>
                <div className={classes.dataGridContainer}>
                    <div style={{ flexGrow: 1 }}>
                        <DataGrid
                            rows={trades}
                            columns={columns}
                            showToolbar={true}
                            autoPageSize={true}
                        />
                    </div>
                </div>
            </Paper>
        </DashboardLayout>
    );
}

