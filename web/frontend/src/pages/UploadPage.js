import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDropzone } from 'react-dropzone';
import {
  Box,
  Button,
  Container,
  Typography,
  Slider,
  TextField,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Alert,
  CircularProgress,
  Paper,
  Grid,
  Stack,
} from '@mui/material';
import CloudUploadIcon from '@mui/icons-material/CloudUpload';
import SettingsIcon from '@mui/icons-material/Settings';
import GlassCard from '../components/GlassCard';
import axios from 'axios';

const UploadPage = () => {
  const [file, setFile] = useState(null);
  const [proxyFile, setProxyFile] = useState(null);
  const [workers, setWorkers] = useState(1);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);

  const navigate = useNavigate();

  const onDrop = (acceptedFiles) => {
    if (acceptedFiles && acceptedFiles.length > 0) {
      setFile(acceptedFiles[0]);
      setError('');
    }
  };

  const onProxyDrop = (acceptedFiles) => {
    if (acceptedFiles && acceptedFiles.length > 0) {
      setProxyFile(acceptedFiles[0]);
    }
  };

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept: {
      'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': ['.xlsx'],
      'application/vnd.ms-excel': ['.xls'],
    },
    multiple: false,
  });

  const { getRootProps: getProxyRootProps, getInputProps: getProxyInputProps } = useDropzone({
    onDrop: onProxyDrop,
    accept: {
      'text/plain': ['.txt'],
    },
    multiple: false,
  });

  const handleSubmit = async () => {
    if (!file) {
      setError('Please select an Excel file');
      return;
    }

    setIsLoading(true);
    setError('');
    
    const formData = new FormData();
    formData.append('file', file);
    formData.append('workers', workers);
    
    if (proxyFile) {
      formData.append('proxy', proxyFile);
    }
    
    try {
      const response = await axios.post('/api/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      
      const { jobId } = response.data;
      navigate(`/progress/${jobId}`);
    } catch (err) {
      setError(err.response?.data?.error || 'An error occurred during upload');
      setIsLoading(false);
    }
  };

  return (
    <Container maxWidth="md" sx={{ py: 4 }}>
      <GlassCard sx={{ p: 4 }}>
        <Typography variant="h4" gutterBottom align="center" color="primary" fontWeight="bold">
          Account Batch Login
        </Typography>
        
        <Typography variant="body1" align="center" paragraph sx={{ mb: 4 }}>
          Upload your Excel file with account details to process login in batch
        </Typography>
        
        {error && (
          <Alert severity="error" sx={{ mb: 3 }}>
            {error}
          </Alert>
        )}
        
        <Paper
          {...getRootProps()}
          elevation={0}
          sx={{
            p: 5,
            mb: 4,
            border: '2px dashed',
            borderColor: isDragActive ? 'primary.main' : 'divider',
            bgcolor: 'background.default',
            borderRadius: 2,
            textAlign: 'center',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            '&:hover': {
              borderColor: 'primary.main',
              bgcolor: 'background.paper',
            },
          }}
        >
          <input {...getInputProps()} />
          <CloudUploadIcon color="primary" sx={{ fontSize: 60, mb: 2 }} />
          <Typography variant="h6" color="textPrimary" gutterBottom>
            {isDragActive ? 'Drop the file here' : 'Drag & drop Excel file here'}
          </Typography>
          <Typography variant="body2" color="textSecondary">
            or click to browse files
          </Typography>
          {file && (
            <Box sx={{ mt: 2, p: 1, bgcolor: 'background.paper', borderRadius: 1 }}>
              <Typography variant="body2" color="primary">
                Selected: {file.name} ({(file.size / 1024).toFixed(2)} KB)
              </Typography>
            </Box>
          )}
        </Paper>
        
        <Box sx={{ mb: 4 }}>
          <Button
            startIcon={<SettingsIcon />}
            onClick={() => setShowAdvanced(!showAdvanced)}
            sx={{ mb: 2 }}
          >
            {showAdvanced ? 'Hide' : 'Show'} Advanced Settings
          </Button>
          
          {showAdvanced && (
            <GlassCard sx={{ p: 3, mt: 2 }}>
              <Grid container spacing={3}>
                <Grid item xs={12}>
                  <Typography gutterBottom>
                    Number of Workers: {workers}
                  </Typography>
                  <Slider
                    value={workers}
                    min={1}
                    max={10}
                    step={1}
                    marks
                    onChange={(e, newValue) => setWorkers(newValue)}
                    valueLabelDisplay="auto"
                  />
                </Grid>
                
                <Grid item xs={12}>
                  <Typography gutterBottom>
                    Proxy File (Optional):
                  </Typography>
                  <Paper
                    {...getProxyRootProps()}
                    elevation={0}
                    sx={{
                      p: 2,
                      border: '1px dashed',
                      borderColor: 'divider',
                      bgcolor: 'background.default',
                      borderRadius: 1,
                      textAlign: 'center',
                      cursor: 'pointer',
                    }}
                  >
                    <input {...getProxyInputProps()} />
                    <Typography variant="body2" color="textSecondary">
                      Drop proxy.txt file here or click to browse
                    </Typography>
                    {proxyFile && (
                      <Typography variant="body2" color="primary" sx={{ mt: 1 }}>
                        Selected: {proxyFile.name}
                      </Typography>
                    )}
                  </Paper>
                </Grid>
              </Grid>
            </GlassCard>
          )}
        </Box>
        
        <Button
          variant="contained"
          size="large"
          fullWidth
          onClick={handleSubmit}
          disabled={!file || isLoading}
          sx={{ py: 1.5 }}
        >
          {isLoading ? <CircularProgress size={24} color="inherit" /> : 'Start Processing'}
        </Button>
      </GlassCard>
    </Container>
  );
};

export default UploadPage;